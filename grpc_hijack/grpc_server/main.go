package main

import (
	"context"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"unsafe"

	"github.com/creack/pty"
	"github.com/gliderlabs/ssh"

	pb "github.com/CustomPoint/snippets/grpc_hijack/hijack"
)

const (
	gRPCServerAddress = "localhost"
	gRPCServerPort = "9990"
	SSHServerAddress = "localhost"
	SSHServerPort = "2222"
)

// server is used to implement hijack.GreeterServer.
type server struct {
	pb.UnimplementedHTServiceServer
}

// HTHello
func (s *server) HTHello(ctx context.Context, in *pb.HelloRequest) (*pb.HelloReply, error) {
	log.Printf("[hi] Received: %v", in.GetName())
	return &pb.HelloReply{Name: "Hello " + in.GetName()}, nil
}

// HTunnel
func (s *server) HTunnel(tunnelServer pb.HTService_HTunnelServer) error {
	// depict server sent over by the tunnelServer
	md, ok := metadata.FromIncomingContext(tunnelServer.Context())
	if !ok {
		return fmt.Errorf("[server-tun] unable to get metadata from context")
	}
	connectIpList := md.Get("connect_ip")
	connectPortList := md.Get("connect_port")
	if len(connectIpList) < 1 || len(connectPortList) < 1 {
		return fmt.Errorf("[server-tun] expected connect_ip & connect_port in metadata")
	}
	connectIp := connectIpList[0]
	connectPort := connectPortList[0]
	addr := net.JoinHostPort(connectIp, connectPort)

	// TCP dial to the intended server
	log.Printf("[server-tun] Connecting to: %v", addr)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		log.Fatalf("[server-tun] Error connecting: %v", err)
		return err
	}
	defer conn.Close()
	defer log.Printf("[server-tun] Connection closed to %v", addr)

	// create channel for error
	errChan := make(chan error)

	// Writing loop
	// - get data from gRPC Client stream
	// - write the received data to the TCP connection handled by target server
	go func() {
		for {
			// receive data from tunnelServer
			dataReply, err := tunnelServer.Recv()
			if err != nil {
				if err != io.EOF {
					log.Printf("[server-tun] Error while receiving data: %v", err)
				}
				errChan <- nil
				return
			}
			data := dataReply.Data

			// send data from tunnelServer to the target server
			log.Printf("[server-tun] Sending bytes to tcp server: %v", len(data))
			_, err = conn.Write(data)
			if err != nil {
				errChan <- fmt.Errorf("[server-tun] unable to write to connection: %v", err)
				return
			}
		}
	}()

	// Reading loop
	// - read data coming in from the target server
	// - write the received data to the stream directed handled by gRPC Client
	go func() {
		buff := make([]byte, 10000)
		for {
			bytesRead, err := conn.Read(buff)
			if err != nil {
				if err != io.EOF {
					log.Printf("[server-tun] Error while receiving data: %v", err)
				} else {
					log.Printf("[server-tun] Remote connection closed")
				}
				errChan <- nil
				return
			}

			log.Printf("[server-tun] Sending bytes to grpc tunnelServer: %v", bytesRead)
			err = tunnelServer.Send(&pb.HReply{
				Data: buff[0:bytesRead],
			})

			if err != nil {
				errChan <- err
				return
			}
		}
	}()

	returnedError := <-errChan
	return returnedError
}

func StartSSHServer() {
	ssh.Handle(func(session ssh.Session) {
		log.Printf("[ssh] Starting SSH Session: %+v", session)
		cmd := exec.Command("sh")
		ptyReq, winCh, isPty := session.Pty()
		if isPty {
			cmd.Env = append(cmd.Env, fmt.Sprintf("TERM=%s", ptyReq.Term))
			f, err := pty.Start(cmd)
			if err != nil {
				panic(err)
			}
			go func() {
				for win := range winCh {
					setWinsize(f, win.Width, win.Height)
				}
			}()
			go func() {
				_, _ = io.Copy(f, session) // stdin
			}()
			_, _ = io.Copy(session, f) // stdout
			_ = cmd.Wait()
		} else {
			_, _ = io.WriteString(session, "No PTY requested.\n")
			_ = session.Exit(1)
		}
	})
	sshServer := net.JoinHostPort(SSHServerAddress, SSHServerPort)
	log.Printf("[ssh] Starting SSH server ...")
	err := ssh.ListenAndServe(sshServer, nil)
	if err != nil {
		log.Fatalf("[ssh] Error from SSH Server: %v", err)
	}

}

func setWinsize(f *os.File, w, h int) {
	_, _, _ = syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), uintptr(syscall.TIOCSWINSZ),
		uintptr(unsafe.Pointer(&struct{ h, w, x, y uint16 }{uint16(h), uint16(w), 0, 0})))
}

func main() {
	var wg sync.WaitGroup
	// Start gRPC server
	wg.Add(1)
	go func() {
		log.Printf("[grpc] Starting the gRPC Server ...")
		lis, err := net.Listen("tcp", net.JoinHostPort(gRPCServerAddress, gRPCServerPort))
		if err != nil {
			log.Fatalf("[srv] failed to listen: %v", err)
		}
		s := grpc.NewServer()
		pb.RegisterHTServiceServer(s, &server{})
		if err := s.Serve(lis); err != nil {
			log.Fatalf("[srv] failed to serve: %v", err)
		}
		wg.Done()
	}()

	// Start the SSH Server
	wg.Add(1)
	go func() {
		StartSSHServer()
		wg.Done()
	}()

	wg.Wait()
}
