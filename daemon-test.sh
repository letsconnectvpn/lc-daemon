#!/bin/sh

# Build Phase
echo "======BUILD======"
go build -o mock/openvpn-sim mock/openvpn-management-sim/main.go
if [ -x "mock/openvpn-sim" ]; then 
    echo "Build OpenVPN-Simulator successful"
else
    echo "Error building OpenVPN-Simulator" 
    exit 1
fi

go build -o  mock/vpn-daemon vpn-daemon/main.go
if [ -x "mock/vpn-daemon" ]; then
    echo "Build VPN-daemon successful"
else
    echo "Error building VPN-daemon"
    pkill openvpn-sim
    exit 1
fi

# Launch Phase
echo "======LAUNCH======"
mock/vpn-daemon &
mock/openvpn-sim &

sleep 1

if ! pgrep -x "vpn-daemon" > /dev/null
then
    echo "VPN-Daemon failed to launch"
    exit 1
fi

if ! pgrep -x "openvpn-sim" > /dev/null
then
    echo "OpenVPN-Simulator failed to launch"
    exit 1
fi

DAEMON_PID="$(pidof vpn-daemon)"
SIM_PID="$(pidof openvpn-sim)"

echo "VPN-daemon launched and running.  PID: $DAEMON_PID"
echo "OpenVPN-sim launched and running. PID: $SIM_PID"

# Test Phase
echo "======TEST======"
php mock/php/vpn-daemon-client.php
EXIT_CODE=$?
if [ ! $EXIT_CODE -eq 0 ]; then
    echo "Test failed. Check last command send."
else
    echo "END TEST: SUCCESSFUL"
fi

# Cleanup Phase
kill $DAEMON_PID
kill $SIM_PID

echo "=====END OF SCRIPT======"
