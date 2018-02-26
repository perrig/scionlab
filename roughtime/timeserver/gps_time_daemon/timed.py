#!/usr/bin/python3

import socket
import sys
import os
import getopt
from timeserver import TimeServer

def ServeRequests(socketFile, timeServer):
    if os.path.exists(socketFile):
        os.remove(socketFile)

    server = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
    server.bind(socketFile)

    server.listen(1)

    while True:
        connection, client_address = server.accept()
        try:
            utcTime=timeServer.get_current_time()
            connection.sendall(utcTime.to_bytes(8, 'little'))
        finally:
            connection.close()


if __name__ == "__main__":
    
    try:
        opts, args = getopt.getopt(sys.argv[1:], "l:h:p:", ["listen_socket=", "brickd_host=", "brickd_port="])
    except getopt.GetoptError as err:
        # print help information and exit:
        print(str(err))   # will print something like "option -a not recognized"
        usage()
        sys.exit(2)
    socket_file = "/run/gps_time_daemon"
    brickd_host = "localhost"
    brickd_port = "4223"

    for o, a in opts:
        if o in ("-l", "--listen_socket"):
            socket_file=a
        elif o in ("-h", "--brickd_host"):
            brickd_host=a
        elif o in ("-p", "--brickd_port"):
            brickd_port=a

    print("Starting GPS time daemon on %s " % (socket_file))

    ts=TimeServer(brickd_host, brickd_port)
    ServeRequests(socket_file, ts)

    input('Press key to exit\n')


