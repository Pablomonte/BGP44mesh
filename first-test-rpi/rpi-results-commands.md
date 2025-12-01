sudo docker exec isp-bird birdc show protocols
BIRD 2.0.12 ready.
Name       Proto      Table      State  Since         Info
device1    Device     ---        up     23:17:00.293  
kernel1    Kernel     master4    up     23:17:00.293  
isp_routes Static     master4    up     23:17:00.293  
customer   BGP        ---        up     01:47:01.248  Established   

sudo docker exec isp-bird birdc show route protocol customer
BIRD 2.0.12 ready.
Table master4:
44.30.127.0/24       unicast [customer 01:47:02.219] ! (100) [AS65000i]
	via 172.30.0.100 on eth0

sudo docker exec isp-bird birdc show route
BIRD 2.0.12 ready.
Table master4:
198.51.100.0/24      blackhole [isp_routes 23:17:00.293] ! (200)
192.0.2.0/24         blackhole [isp_routes 23:17:00.293] ! (200)
44.30.127.0/24       unicast [customer 01:47:02.219] ! (100) [AS65000i]
	via 172.30.0.100 on eth0
203.0.113.0/24       blackhole [isp_routes 23:17:00.293] ! (200)

 ip route
default via 192.168.1.1 dev wlan0 proto dhcp src 192.168.1.56 metric 600 
44.30.127.0/24 via 172.30.0.100 dev eth0 
172.17.0.0/16 dev docker0 proto kernel scope link src 172.17.0.1 linkdown 
172.30.0.0/24 dev eth0 proto kernel scope link src 172.30.0.1 
192.168.1.0/24 dev wlan0 proto kernel scope link src 192.168.1.56 metric 600 

ip route | grep 44.30
44.30.127.0/24 via 172.30.0.100 dev eth0 

ping -c 3 172.30.0.100
PING 172.30.0.100 (172.30.0.100) 56(84) bytes of data.
64 bytes from 172.30.0.100: icmp_seq=1 ttl=64 time=0.294 ms
64 bytes from 172.30.0.100: icmp_seq=2 ttl=64 time=0.904 ms
64 bytes from 172.30.0.100: icmp_seq=3 ttl=64 time=0.276 ms

--- 172.30.0.100 ping statistics ---
3 packets transmitted, 3 received, 0% packet loss, time 2032ms
rtt min/avg/max/mdev = 0.276/0.491/0.904/0.291 ms

ping -c 3 44.30.127.1
PING 44.30.127.1 (44.30.127.1) 56(84) bytes of data.
64 bytes from 44.30.127.1: icmp_seq=1 ttl=64 time=0.389 ms
64 bytes from 44.30.127.1: icmp_seq=2 ttl=64 time=0.288 ms
64 bytes from 44.30.127.1: icmp_seq=3 ttl=64 time=0.245 ms

--- 44.30.127.1 ping statistics ---
3 packets transmitted, 3 received, 0% packet loss, time 2044ms
rtt min/avg/max/mdev = 0.245/0.307/0.389/0.060 ms

ping -c 3 44.30.127.2
PING 44.30.127.2 (44.30.127.2) 56(84) bytes of data.
64 bytes from 44.30.127.2: icmp_seq=1 ttl=63 time=1.69 ms
64 bytes from 44.30.127.2: icmp_seq=2 ttl=63 time=1.68 ms
64 bytes from 44.30.127.2: icmp_seq=3 ttl=63 time=1.65 ms

--- 44.30.127.2 ping statistics ---
3 packets transmitted, 3 received, 0% packet loss, time 2003ms
rtt min/avg/max/mdev = 1.647/1.669/1.686/0.016 ms

ip addr show | grep -E "inet |: <"
1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue state UNKNOWN group default qlen 1000
    inet 127.0.0.1/8 scope host lo
2: eth0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc fq_codel state UP group default qlen 1000
    inet 172.30.0.1/24 brd 172.30.0.255 scope global eth0
3: wlan0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc fq_codel state UP group default qlen 1000
    inet 192.168.1.56/24 brd 192.168.1.255 scope global dynamic noprefixroute wlan0
4: docker0: <NO-CARRIER,BROADCAST,MULTICAST,UP> mtu 1500 qdisc noqueue state DOWN group default 
    inet 172.17.0.1/16 brd 172.17.255.255 scope global docker0

ip neigh show
192.168.1.1 dev wlan0 lladdr f0:c4:78:71:bc:43 REACHABLE 
172.30.0.101 dev eth0 lladdr d0:c0:bf:2f:5e:29 STALE 
192.168.1.16 dev wlan0 lladdr c0:bf:be:e3:8c:7e REACHABLE 
172.30.0.99 dev eth0 lladdr 28:c5:c8:d5:46:d4 STALE 
172.30.0.100 dev eth0 lladdr da:85:00:40:a5:96 REACHABLE 
fe80::1 dev wlan0 lladdr f0:c4:78:71:bc:43 router STALE 
