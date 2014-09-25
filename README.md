# sysinfo_influxdb

A collecting tool of system metrics (CPU, Memory, Load, disks I/Os, Network traffic) to an [InfluxDB](http://influxdb.org) server.

## Release

You can download the lastest version for [GNU/Linux amd64 here](https://github.com/novaquark/sysinfo_influxdb/releases/download/0.3.0/sysinfo_influxdb).

## Usage sample

To display all metrics without sending them to a server :

    $GOPATH/bin/sysinfo_influxdb

To send metric to an InfluxDB server, only one time :

    $GOPATH/bin/sysinfo_influxdb -h localhost:8086 -u root -p secret -d database

You can ommit `-h`, `-u` and `-p` if you use default settings.

By default, table are prefixed by the hostname, to change or disable the prefix, just do :

    $GOPATH/bin/sysinfo_influxdb -P "" # Disable prefix
    $GOPATH/bin/sysinfo_influxdb -P "koala"

To append FQDN of the running host at the end of injected data, use the `--fqdn` flag :

    $GOPATH/bin/sysinfo_influxdb --fqdn

To remove `cpu` row (sum of cpu0, cpu1, ...) from cpu metrics, use the `--nocpusum` flag :

    $GOPATH/bin/sysinfo_influxdb --nocpusum

To run in daemon mode (doesn't fork, just loop), use the `-D` option :

    $GOPATH/bin/sysinfo_influxdb -D

To display data even if you send them to a server, use `-v` :

    $GOPATH/bin/sysinfo_influxdb -D -h localhost:8086 -d database -v

Use the `-i` option to change the collect interval; this option alters the consistency of quantities displayed or sent (CPUs, network or disks I/Os) : the amount of outgoing packets in 1 minute is not directly comparable to the same amount outgoing in 1 second when saved in the same table. For example, to collect statistics each minute :

    $GOPATH/bin/sysinfo_influxdb -i 1m

To change data collected, use the `-c` option with one or more metrics type (`cpus`, `mem`, `swap`, `uptime`, `load`, `network`, `disks`) like this :

    $GOPATH/bin/sysinfo_influxdb -c cpus # Collect only CPUs related statistics
    $GOPATH/bin/sysinfo_influxdb -c load,cpus,disks # Collect load average, CPUs and disks I/Os statistics

On hardened kernel, you must be allowed to read `/proc/net/dev` in order to collect networking statistics.

## Output format

### CPUs

#### Text

	#0: koala.cpu
	| id	| user	| nice	| sys	| idle	| wait	| total	|
	| cpu	| 4	| 4	| 2	| 794	| 3	| 807	|
	| cpu0	| 1	| 1	| 1	| 95	| 2	| 100	|
	| cpu1	| 1	| 0	| 1	| 99	| 0	| 101	|
	| cpu2	| 1	| 0	| 0	| 99	| 0	| 100	|
	| cpu3	| 1	| 1	| 1	| 99	| 0	| 102	|

#### JSON

	[{"name":"koala.cpu","columns":["id","user","nice","sys","idle","wait","total"],"points":[["cpu",3,0,1,795,0,799],["cpu0",0,0,0,99,0,99],["cpu1",0,0,1,99,0,100],["cpu2",1,0,0,99,0,100],["cpu3",1,0,1,98,0,100]]}]

### Memory

#### Text

	#0: koala.mem
	| free		| used		| actualfree	| actualused	| total		|
	| 2004123648	| 6305091584	| 3761774592	| 4547440640	| 8309215232	|

#### JSON

	[{"name":"koala.mem","columns":["free","used","actualfree","actualused","total"],"points":[[2004123648,6305091584,3761774592,4547440640,8309215232]]}]

### Swap

#### Text

	#0: koala.swap
	| free		| used	| total		|
	| 8589930496	| 0	| 8589930496	|


#### JSON

	[{"name":"koala.swap","columns":["free","used","total"],"points":[[8589930496,0,8589930496]]}]

### Uptime

#### Text

	#0: koala.uptime
	| length	|
	| 285235	|

#### JSON

	[{"name":"koala.uptime","columns":["length"],"points":[[285235]]}]

### Load average

#### Text

	#0: koala.load
	| one	| five	| fifteen	|
	| 0.19	| 0.17	| 0.15		|

#### JSON

	[{"name":"koala.load","columns":["one","five","fifteen"],"points":[[0.19,0.17,0.15]]}]


### Network

#### Text

	#0: koala.network
	| iface	| recv_bytes	| recv_packets	| recv_errs	| recv_drop	| recv_fifo	| recv_frame	| recv_compressed | recv_multicast | trans_bytes | trans_packets | trans_errs | trans_drop | trans_fifo | trans_colls | trans_carrier | trans_compressed |
	| br0	| 1934		| 16		| 0		| 0		| 0		| 0		| 0		  | 0		   | 2592	 | 20		 | 0	      | 0	   | 0		| 0	      | 0	      | 0		 |
	| vnet1	| 0		| 0		| 0		| 0		| 0		| 0		| 0		  | 0		   | 969	 | 8		 | 0	      | 0	   | 0		| 0	      | 0	      | 0		 |
	| eth0	| 2158		| 16		| 0		| 0		| 0		| 0		| 0		  | 0		   | 2644	 | 21		 | 0	      | 0	   | 0		| 0	      | 0	      | 0		 |
	| lo	| 0		| 0		| 0		| 0		| 0		| 0		| 0		  | 0		   | 0		 | 0		 | 0	      | 0	   | 0		| 0	      | 0	      | 0		 |

#### JSON

	[{"name":"koala.network","columns":["iface","recv_bytes","recv_packets","recv_errs","recv_drop","recv_fifo","recv_frame","recv_compressed","recv_multicast","trans_bytes","trans_packets","trans_errs","trans_drop","trans_fifo","trans_colls","trans_carrier","trans_compressed"],"points":[["br0",2461,22,0,0,0,0,0,0,2674,21,0,0,0,0,0,0],["vnet1",0,0,0,0,0,0,0,0,1572,12,0,0,0,0,0,0],["eth0",2769,22,0,0,0,0,0,0,2674,21,0,0,0,0,0,0],["lo",0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0]}]

### Disks I/Os

#### Text

	#0: koala.disks
	| device    | read_ios	| read_merges	| read_sectors	| read_ticks	| write_ios	| write_merges	| write_sectors	| write_ticks | in_flight | io_ticks | time_in_queue |
	| sda	    | 0		| 0		| 0		| 0		| 0		| 0		| 0		| 0	      | 0	  | 0	     | 0	     |
	| sda1	    | 0		| 0		| 0		| 0		| 0		| 0		| 0		| 0	      | 0	  | 0	     | 0	     |
	| sda2	    | 0		| 0		| 0		| 0		| 0		| 0		| 0		| 0	      | 0	  | 0	     | 0	     |

#### JSON

	[{"name":"koala.disks","columns":["device","read_ios","read_merges","read_sectors","read_ticks","write_ios","write_merges","write_sectors","write_ticks","in_flight","io_ticks","time_in_queue"],"points":[["sda",0,0,0,0,0,0,0,0,0,0,0],["sda1",0,0,0,0,0,0,0,0,0,0,0],["sda2",0,0,0,0,0,0,0,0,0,0,0]]}]

## Building

	cd $GOPATH
	mkdir -p src/github.com/novaquark/
	cd src/github.com/novaquark/
	git clone https://github.com/novaquark/sysinfo_influxdb.git
	cd sysinfo_influxdb
	go get
	go install

<p xmlns:dct="http://purl.org/dc/terms/">
  <a rel="license"
     href="http://creativecommons.org/publicdomain/zero/1.0/">
    <img src="http://i.creativecommons.org/p/zero/1.0/88x31.png" style="border-style: none;" alt="CC0" />
  </a>
  <br />
  To the extent possible under law,
  <a rel="dct:publisher"
     href="https://github.com/orgs/novaquark">
    <span property="dct:title">Novaquark</span></a>
  has waived all copyright and related or neighboring rights to
  this work.
</p>
