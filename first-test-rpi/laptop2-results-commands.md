docker exec tinc2 tail -30 /var/run/tinc/bgpmesh/tinc.log | grep -E "PING|PONG|node1" | tail -10
2025-11-30 02:05:47 tinc[1]: Sending PING to node1 (172.30.0.100 port 655)
2025-11-30 02:05:47 tinc[1]: Got PONG from node1 (172.30.0.100 port 655)
2025-11-30 02:06:46 tinc[1]: Got PING from node1 (172.30.0.100 port 655)
2025-11-30 02:06:46 tinc[1]: Sending PONG to node1 (172.30.0.100 port 655)
2025-11-30 02:06:47 tinc[1]: Sending PING to node1 (172.30.0.100 port 655)
2025-11-30 02:06:47 tinc[1]: Got PONG from node1 (172.30.0.100 port 655)
2025-11-30 02:07:46 tinc[1]: Got PING from node1 (172.30.0.100 port 655)
2025-11-30 02:07:46 tinc[1]: Sending PONG to node1 (172.30.0.100 port 655)
2025-11-30 02:07:47 tinc[1]: Sending PING to node1 (172.30.0.100 port 655)
2025-11-30 02:07:47 tinc[1]: Got PONG from node1 (172.30.0.100 port 655)

docker exec tinc2 ip addr show | grep -E "inet |: <"
1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue state UNKNOWN group default qlen 1000
    inet 127.0.0.1/8 scope host lo
2: eth0@if30: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UP group default 
    inet 172.23.0.3/16 brd 172.23.255.255 scope global eth0
3: eth1@if31: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UP group default 
    inet 172.22.0.3/16 brd 172.22.255.255 scope global eth1
4: tinc0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1400 qdisc fq_codel state UNKNOWN group default qlen 1000
    inet 44.30.127.2/24 scope global tinc0

docker exec tinc2 ip route
default via 172.22.0.1 dev eth1 
44.30.127.0/24 dev tinc0 proto kernel scope link src 44.30.127.2 
172.22.0.0/16 dev eth1 proto kernel scope link src 172.22.0.3 
172.23.0.0/16 dev eth0 proto kernel scope link src 172.23.0.3 
172.30.0.1 via 44.30.127.1 dev tinc0 

docker exec tinc2 ip neigh show dev tinc0
44.30.127.1 lladdr 1e:c4:83:df:5d:e8 REACHABLE 

docker exec tinc2 ping -c 3 44.30.127.1
PING 44.30.127.1 (44.30.127.1) 56(84) bytes of data.
64 bytes from 44.30.127.1: icmp_seq=1 ttl=64 time=0.682 ms
64 bytes from 44.30.127.1: icmp_seq=2 ttl=64 time=1.45 ms
64 bytes from 44.30.127.1: icmp_seq=3 ttl=64 time=1.32 ms

--- 44.30.127.1 ping statistics ---
3 packets transmitted, 3 received, 0% packet loss, time 2027ms
rtt min/avg/max/mdev = 0.682/1.151/1.453/0.336 ms

docker exec tinc2 ping -c 3 172.30.0.1
PING 172.30.0.1 (172.30.0.1) 56(84) bytes of data.
64 bytes from 172.30.0.1: icmp_seq=1 ttl=63 time=1.25 ms
64 bytes from 172.30.0.1: icmp_seq=2 ttl=63 time=1.56 ms
64 bytes from 172.30.0.1: icmp_seq=3 ttl=63 time=2.09 ms

--- 172.30.0.1 ping statistics ---
3 packets transmitted, 3 received, 0% packet loss, time 2003ms
rtt min/avg/max/mdev = 1.247/1.632/2.093/0.349 ms

docker exec tinc2 traceroute -n 172.30.0.1
OCI runtime exec failed: exec failed: unable to start container process: exec: "traceroute": executable file not found in $PATH

docker exec tinc2 ping -c 2 192.0.2.1
PING 192.0.2.1 (192.0.2.1) 56(84) bytes of data.

--- 192.0.2.1 ping statistics ---
2 packets transmitted, 0 received, 100% packet loss, time 1025ms
