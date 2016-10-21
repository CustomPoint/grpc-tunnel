### Intended to have an eye on the new releases for different OSes

import os
import re
import feedparser
import pickle
import urllib3
from pprint import pprint

URLS = ["http://distrowatch.com/news/dwd.xml"]
PLATFORMS = ["centos", "red hat", "rhel", "oel", "ubuntu", "suse", "arch"]
DATABASE_FILE = "distro_data"
PROXY = urllib3.ProxyManager("")

def get_parsed_object(url):
    """ Returns the dict object parsed from the RSS feed url """
    os.environ["http_proxy"] = ""
    return feedparser.parse(url)


def open_database_file():
    if not os.path.exists(DATABASE_FILE):
        print('WARNING : Database file "{0}" was not found !!'.format(DATABASE_FILE))
        return []
    fh = open(DATABASE_FILE, "rb")
    obj = pickle.load(fh)
    fh.close()
    return obj


def write_database_file(obj):
    fh = open(DATABASE_FILE, "wb")
    pickle.dump(obj, fh)


def update_database(entries):
    if not entries:
        print("WARNING > There were no articles found.")
        return
    data = open_database_file()
    titles = [elem['title'] for elem in data]
    new_entries = [elem for elem in entries if elem['title'] not in titles]
    if new_entries:
        data.extend(new_entries)
        write_database_file(data)
    else:
        print("WARNING > There were no new entries.")


def print_database():
    data = open_database_file()
    print('=' * 50)
    print('====== The DATABASE')
    for elem in data:
        print('==== Distro: ' + elem['title'])
        print('== Link: ' + elem['link'])
        print('== Summary: ' + elem['summary'])
    print('=' * 50)


def check_entries(entries):
    """ Returns the entries that match PLATFORMS checks """
    new_entries = []
    for entry in entries:
        for plat in PLATFORMS:
            if re.match(".*" + plat + ".*", entry["title"], re.IGNORECASE):
                print('New entry found > {0} [{1}] - Summary: {2}'.format(entry["title"], entry["link"], entry["summary"]))
                new_entries.append({'title': entry["title"], 'link': entry["link"], 'summary': entry["summary"]})
    return new_entries


print('Starting getting the feed ... ')
for url in URLS:
    obj = get_parsed_object(url)
    print('<DEBUG>')
    pprint(obj)
    print('</DEBUG>')
    print("Object in place. Checking entries ...")
    entries = check_entries(obj['entries'])
    print()
    update_database(entries)
print('DONE')
print('-'*75)
print_database()

