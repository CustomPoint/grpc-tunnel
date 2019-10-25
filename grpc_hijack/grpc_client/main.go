package main

import (
	"bufio"
	"context"
	"fmt"
	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"time"

	pb "github.com/CustomPoint/snippets/grpc_hijack/hijack"
	"github.com/shiena/ansicolor"
)

const (
	gRPCClientAddress = "localhost:9990"
	gRPCClientName    = "client"
	tcpType           = "tcp"
	tcpHost           = "localhost"
	tcpPort           = "9991"
)

func SayHi(c pb.HTServiceClient) {
	// Contact the server and print out its response.
	name := gRPCClientName
	if len(os.Args) > 1 {
		name = os.Args[1]
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	r, err := c.HTHello(ctx, &pb.HelloRequest{Name: name})
	if err != nil {
		log.Fatalf("[hi] could not greet: %v", err)
	}
	log.Printf("[hi] Greeting: %s", r.GetName())
}

func Tunnel(gRPCClient pb.HTServiceClient, tcpConn net.Conn) {
	md := metadata.Pairs(
		"connect_ip", "localhost",
		"connect_port", "2222",
	)
	ctx := metadata.NewOutgoingContext(context.Background(), md)
	log.Printf("[client-tun] Connection details: %+v", md)

	tunnelClient, err := gRPCClient.HTunnel(ctx)
	if err != nil {
		log.Fatalf("[client-tun] could not create client-tunnel: %+v", err)
	}

	// Writing loop
	// - get data from gRPC Server Stream
	// - write data to the TCP connection
	go func() {
		for {
			// receive data from gRPC Stream
			chunk, err := tunnelClient.Recv()
			if err != nil {
				log.Fatalf("[client-tun] Recv terminated: %+v", err)
			}

			// write that data directly to the TCP connection
			_, err = tcpConn.Write(chunk.Data)
			if err != nil {
				log.Fatalf("[client-tun] Writing back failed: %+v", err)
			}
		}
	}()

	// Reading loop
	// - read data from TCP connection
	// - send data over the gRPC stream
	buf := make([]byte, 0, 4*1024)
	for {
		// receive data from TCP connection
		n, err := tcpConn.Read(buf[:cap(buf)])
		buf = buf[:n]
		if n == 0 {
			if err == nil {
				continue
			}
			if err == io.EOF {
				break
			}
			log.Fatalf("[client-tun] Error while processing buf: %v", err)
		}
		// process buf
		if err != nil && err != io.EOF {
			log.Fatalf("[client-tun] Error while processing buf: %v", err)
		}

		// write data to the gRPC stream
		err = tunnelClient.Send(&pb.HRequest{Data: buf})
		if err != nil {
			log.Fatalf("[client-tun] Error while sending buf: %v", err)
		}
	}
}

func StartTCPServer(gRPCClient pb.HTServiceClient) {
	// Listen for incoming connections.
	l, err := net.Listen(tcpType, net.JoinHostPort(tcpHost, tcpPort))
	if err != nil {
		log.Fatalf("[tcp] Error listening: %+v", err.Error())
	}
	// Close the listener when the application closes.
	defer l.Close()
	log.Printf("[tcp] Listening on " + net.JoinHostPort(tcpHost, tcpPort))
	for {
		// Listen for an incoming connection.
		conn, err := l.Accept()
		if err != nil {
			log.Fatalf("[tcp] Error accepting: %+v", err.Error())
		}
		// Handle connections in a new goroutine.
		go Tunnel(gRPCClient, conn)
	}
}

func StartSSHClient() {
	sshConfig := &ssh.ClientConfig{
		User: "user",
		Auth: []ssh.AuthMethod{
			ssh.Password("pass"),
		},
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
		Timeout: time.Second * 10,
	}
	log.Printf("SSH Creating the Client: \n")
	conn, err := net.Dial(tcpType, net.JoinHostPort(tcpHost, tcpPort))
	if err != nil {
		log.Fatalf("[error] Dialed to server error: %+v", err)
	}

	c, chans, reqs, err := ssh.NewClientConn(conn, net.JoinHostPort(tcpHost, tcpPort), sshConfig)
	if err != nil {
		log.Fatalf("[error] new client conn !")
	}
	sshClient := ssh.NewClient(c, chans, reqs)
	session, err := sshClient.NewSession()
	if err != nil {
		log.Fatalf("SSH Failed to create session: " + err.Error())
	}
	defer session.Close()

	// Set IO
	session.Stdout = ansicolor.NewAnsiColorWriter(os.Stdout)
	session.Stderr = ansicolor.NewAnsiColorWriter(os.Stderr)
	in, _ := session.StdinPipe()
	modes := ssh.TerminalModes{
		ssh.ECHO:  0, // Disable echoing
		ssh.IGNCR: 1, // Ignore CR on input.
	}
	// Request pseudo terminal
	if err := session.RequestPty("xterm", 200, 200, modes); err != nil {
		log.Fatalf("request for pseudo terminal failed: %s", err)
	}
	// Start remote shell
	if err := session.Shell(); err != nil {
		log.Fatalf("failed to start shell: %s", err)
	}
	// Accepting commands
	log.Printf("SSH Entering forever loop for commands... \n")
	for {
		reader := bufio.NewReader(os.Stdin)
		str, _ := reader.ReadString('\n')
		_, _ = fmt.Fprint(in, str)
	}
}

func main() {
	var wg sync.WaitGroup
	// Start gRPC client
	log.Printf("[grpc] Dialing to the gRPC Server ...")
	conn, err := grpc.Dial(gRPCClientAddress, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		log.Fatalf("[srv] did not connect: %v", err)
	}
	log.Printf("[grpc] Connection established")
	defer conn.Close()

	// create gRPC client
	gRPCClient := pb.NewHTServiceClient(conn)

	// Start hidden TCP Server
	wg.Add(1)
	go func() {
		StartTCPServer(gRPCClient)
		wg.Done()
	}()

	// Start SSH client
	wg.Add(1)
	go func() {
		// sleep so that the TCP Server is up & running
		// TODO: a better retry solution
		time.Sleep(3)
		StartSSHClient()
		wg.Done()
	}()

	wg.Wait()
}
