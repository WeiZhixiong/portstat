# portstat

Monitor the number of available tcp ports <br/>
remote users requesting connections on the local listening port are ignored.

## Usage
- Default outputs the top 10 smallest available ports
- Default interval is 3 seconds

```bash
> portstat
Connect                                                                                            UsedPorts  AvailablePorts
192.168.170.132->192.168.170.132:22                                                                2          28229
127.0.0.1->127.0.0.1:22                                                                            1          28230
```

```bash
> portstat --prom
tcp_used_ports_total{connect="192.168.170.132->192.168.170.132:22"} 2
tcp_available_ports_total{connect="192.168.170.132->192.168.170.132:22"} 28229
tcp_used_ports_total{connect="127.0.0.1->127.0.0.1:22"} 1
tcp_available_ports_total{connect="127.0.0.1->127.0.0.1:22"} 28230                                                                            1          28230
```