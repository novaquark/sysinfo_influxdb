// sysinfo_influxdb by Novaquark
//
// To the extent possible under law, the person who associated CC0 with
// sysinfo_influxdb has waived all copyright and related or neighboring rights
// to sysinfo_influxdb.
//
// You should have received a copy of the CC0 legalcode along with this
// work.  If not, see <http://creativecommons.org/publicdomain/zero/1.0/>.

package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/cloudfoundry/gosigar"
	influxClient "github.com/influxdb/influxdb/client"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const APP_VERSION = "0.5.1"

// Variables storing arguments flags
var verboseFlag string
var versionFlag bool
var daemonFlag bool
var fqdnFlag bool
var daemonIntervalFlag time.Duration
var daemonConsistencyFlag time.Duration
var consistencyFactor = 1.0
var prefixFlag string
var collectFlag string
var pidFile string

var hostFlag string
var usernameFlag string
var passwordFlag string
var secretFlag string
var databaseFlag string

func init() {
	flag.BoolVar(&versionFlag, "version", false, "Print the version number and exit.")
	flag.BoolVar(&versionFlag, "V", false, "Print the version number and exit (shorthand).")

	flag.StringVar(&verboseFlag, "verbose", "", "Display debug information: choose between text or JSON.")
	flag.StringVar(&verboseFlag, "v", "", "Display debug information: choose between text or JSON (shorthand).")

	hostname, _ := os.Hostname()
	flag.StringVar(&prefixFlag, "prefix", hostname, "Change series name prefix.")
	flag.StringVar(&prefixFlag, "P", hostname, "Change series name prefix (shorthand).")

	flag.BoolVar(&fqdnFlag, "fqdn", false, "Append fqdn with every data")
	flag.BoolVar(&fqdnFlag, "f", false, "Append fqdn with every data (shorthand).")

	flag.StringVar(&hostFlag, "host", "localhost:8086", "Connect to host.")
	flag.StringVar(&hostFlag, "h", "localhost:8086", "Connect to host (shorthand).")
	flag.StringVar(&usernameFlag, "username", "root", "User for login.")
	flag.StringVar(&usernameFlag, "u", "root", "User for login (shorthand).")
	flag.StringVar(&passwordFlag, "password", "root", "Password to use when connecting to server.")
	flag.StringVar(&passwordFlag, "p", "root", "Password to use when connecting to server (shorthand).")
	flag.StringVar(&secretFlag, "secret", "", "Absolute path to password file (shorthand). '-p' is ignored if specifed.")
	flag.StringVar(&secretFlag, "s", "", "Absolute path to password file. '-p' is ignored if specifed.")
	flag.StringVar(&databaseFlag, "database", "", "Name of the database to use.")
	flag.StringVar(&databaseFlag, "d", "", "Name of the database to use (shorthand).")

	flag.StringVar(&collectFlag, "collect", "cpu,cpus,mem,swap,uptime,load,network,disks,mounts", "Chose which data to collect.")
	flag.StringVar(&collectFlag, "c", "cpu,cpus,mem,swap,uptime,load,network,disks,mounts", "Chose which data to collect (shorthand).")

	flag.BoolVar(&daemonFlag, "daemon", false, "Run in daemon mode.")
	flag.BoolVar(&daemonFlag, "D", false, "Run in daemon mode (shorthand).")
	flag.DurationVar(&daemonIntervalFlag, "interval", time.Second, "With daemon mode, change time between checks.")
	flag.DurationVar(&daemonIntervalFlag, "i", time.Second, "With daemon mode, change time between checks (shorthand).")
	flag.DurationVar(&daemonConsistencyFlag, "consistency", time.Second, "With custom interval, duration to bring back collected values for data consistency (0s to disable).")
	flag.DurationVar(&daemonConsistencyFlag, "C", time.Second, "With daemon mode, duration to bring back collected values for data consistency (shorthand).")

	flag.StringVar(&pidFile, "pidfile", "", "the pid file")
}

func getFqdn() string {
	// Note: We use exec here instead of os.Hostname() because we
	// want the FQDN, and this is the easiest way to get it.
	fqdn, err := exec.Command("hostname", "-f").Output()

	// Fallback on Unqualifed name
	if err != nil {
		hostname, _ := os.Hostname()
		return hostname
	}

	return strings.TrimSpace(string(fqdn))
}

// "in_array" style func for strings
func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func main() {
	flag.Parse() // Scan the arguments list

	if versionFlag {
		fmt.Println("Version:", APP_VERSION)
	} else {
		if pidFile != "" {
			pid := strconv.Itoa(os.Getpid())
			if err := ioutil.WriteFile(pidFile, []byte(pid), 0644); err != nil {
				fmt.Fprintf(os.Stderr, "Unable to create pidfile\n")
				panic(err)
			}
		}

		if daemonConsistencyFlag.Seconds() > 0 {
			consistencyFactor = daemonConsistencyFlag.Seconds() / daemonIntervalFlag.Seconds()
		}

		// Fill InfluxDB connection settings
		var client *influxClient.Client = nil
		if databaseFlag != "" {
			config := new(influxClient.ClientConfig)

			config.Host = hostFlag
			config.Username = usernameFlag
			config.Database = databaseFlag

			// use secret file if present, fallback to CLI password arg
			if secretFlag != "" {
				data, err := ioutil.ReadFile(secretFlag)
				if err != nil {
					panic(err)
				}
				config.Password = strings.Split(string(data), "\n")[0]
			} else {
				config.Password = passwordFlag
			}

			var err error
			client, err = influxClient.NewClient(config)

			if err != nil {
				panic(err)
			}
		}

		// Build collect list
		var collectList []GatherFunc
		for _, c := range strings.Split(collectFlag, ",") {
			switch strings.Trim(c, " ") {
			case "cpu":
				collectList = append(collectList, cpu)
			case "cpus":
				collectList = append(collectList, cpus)
			case "mem":
				collectList = append(collectList, mem)
			case "swap":
				collectList = append(collectList, swap)
			case "uptime":
				collectList = append(collectList, uptime)
			case "load":
				collectList = append(collectList, load)
			case "network":
				collectList = append(collectList, network)
			case "disks":
				collectList = append(collectList, disks)
			case "mounts":
				collectList = append(collectList, mounts)
			default:
				fmt.Fprintf(os.Stderr, "Unknown collect option `%s'\n", c)
				return
			}
		}

		if prefixFlag != "" && prefixFlag[len(prefixFlag)-1] != '.' {
			prefixFlag += "."
		}

		ch := make(chan *influxClient.Series, len(collectList))

		// Without daemon mode, do at least one lap
		first := true

		for first || daemonFlag {
			first = false

			// Collect data
			var data []*influxClient.Series

			for _, cl := range collectList {
				go cl(prefixFlag, ch)
			}

			for i := len(collectList); i > 0; i-- {
				res := <-ch
				if res != nil {
					data = append(data, res)
				} else if !daemonFlag {
					// Loop if we haven't all data:
					// Since diffed data didn't respond the
					// first time they are collected, loop
					// one more time to have it
					first = true
				}
			}

			if !first {
				if fqdnFlag {
					for _, serie := range data {
						serie.Columns = append(serie.Columns, "fqdn")
						for kv, value := range serie.Points {
							serie.Points[kv] = append(value, getFqdn())
						}
					}
				}

				// Show data
				if !first && (databaseFlag == "" || verboseFlag != "") {
					if strings.ToLower(verboseFlag) == "text" || verboseFlag == "" {
						prettyPrinter(data)
					} else {
						b, _ := json.Marshal(data)
						fmt.Printf("%s\n", b)
					}
				}

				// Send data
				if client != nil {
					if err := send(client, data); err != nil {
						fmt.Fprintf(os.Stderr, "%s\n", err)
					}
				}
			}

			if daemonFlag || first {
				time.Sleep(daemonIntervalFlag)
			}
		}
	}
}

func prettyPrinter(series []*influxClient.Series) {
	for ks, serie := range series {
		nbCols := len(serie.Columns)

		fmt.Printf("\n#%d: %s\n", ks, serie.Name)

		for _, col := range serie.Columns {
			fmt.Printf("| %s\t", col)
		}
		fmt.Println("|")

		for _, value := range serie.Points {
			fmt.Print("| ")
			for i := 0; i < nbCols; i++ {
				fmt.Print(value[i], "\t| ")
			}
			fmt.Print("\n")
		}
	}
}

/**
 * Interactions with InfluxDB
 */

func send(client *influxClient.Client, series []*influxClient.Series) error {
	return client.WriteSeries(series)
}

/**
 * Diff function
 */

var last_series = make(map[string][][]interface{})

func DiffFromLast(serie *influxClient.Series) *influxClient.Series {
	notComplete := false

	if _, ok := last_series[serie.Name]; !ok {
		last_series[serie.Name] = [][]interface{}{}
	}

	for i := 0; i < len(serie.Points); i++ {
		if len(last_series[serie.Name]) <= i {
			last_series[serie.Name] = append(last_series[serie.Name], []interface{}{})
		}
		for j := 0; j < len(serie.Points[i]); j++ {
			var tmp interface{}
			if len(last_series[serie.Name][i]) <= j {
				tmp = serie.Points[i][j]
				notComplete = true
				last_series[serie.Name][i] = append(last_series[serie.Name][i], serie.Points[i][j])
			} else {
				tmp = last_series[serie.Name][i][j]
				last_series[serie.Name][i][j] = serie.Points[i][j]
			}

			switch serie.Points[i][j].(type) {
			case int8:
				serie.Points[i][j] = int8(float64(serie.Points[i][j].(int8)-tmp.(int8)) * consistencyFactor)
			case int16:
				serie.Points[i][j] = int16(float64(serie.Points[i][j].(int16)-tmp.(int16)) * consistencyFactor)
			case int32:
				serie.Points[i][j] = int32(float64(serie.Points[i][j].(int32)-tmp.(int32)) * consistencyFactor)
			case int64:
				serie.Points[i][j] = int64(float64(serie.Points[i][j].(int64)-tmp.(int64)) * consistencyFactor)
			case uint8:
				serie.Points[i][j] = uint8(float64(serie.Points[i][j].(uint8)-tmp.(uint8)) * consistencyFactor)
			case uint16:
				serie.Points[i][j] = uint16(float64(serie.Points[i][j].(uint16)-tmp.(uint16)) * consistencyFactor)
			case uint32:
				serie.Points[i][j] = uint32(float64(serie.Points[i][j].(uint32)-tmp.(uint32)) * consistencyFactor)
			case uint64:
				serie.Points[i][j] = uint64(float64(serie.Points[i][j].(uint64)-tmp.(uint64)) * consistencyFactor)
			case int:
				serie.Points[i][j] = int(float64(serie.Points[i][j].(int)-tmp.(int)) * consistencyFactor)
			case uint:
				serie.Points[i][j] = uint(float64(serie.Points[i][j].(uint)-tmp.(uint)) * consistencyFactor)
			}
		}
	}

	if notComplete {
		return nil
	} else {
		return serie
	}
}

/**
 * Gathering functions
 */

type GatherFunc func(string, chan *influxClient.Series) error

func cpu(prefix string, ch chan *influxClient.Series) error {
	serie := &influxClient.Series{
		Name:    prefix + "cpu",
		Columns: []string{"id", "user", "nice", "sys", "idle", "wait", "total"},
		Points:  [][]interface{}{},
	}

	cpu := sigar.Cpu{}
	if err := cpu.Get(); err != nil {
		ch <- nil
		return err
	}
	serie.Points = append(serie.Points, []interface{}{"cpu", cpu.User, cpu.Nice, cpu.Sys, cpu.Idle, cpu.Wait, cpu.Total()})

	ch <- DiffFromLast(serie)
	return nil
}

func cpus(prefix string, ch chan *influxClient.Series) error {
	serie := &influxClient.Series{
		Name:    prefix + "cpus",
		Columns: []string{"id", "user", "nice", "sys", "idle", "wait", "total"},
		Points:  [][]interface{}{},
	}

	cpus := sigar.CpuList{}
	cpus.Get()
	for i, cpu := range cpus.List {
		serie.Points = append(serie.Points, []interface{}{fmt.Sprint("cpu", i), cpu.User, cpu.Nice, cpu.Sys, cpu.Idle, cpu.Wait, cpu.Total()})
	}

	ch <- DiffFromLast(serie)
	return nil
}

func mem(prefix string, ch chan *influxClient.Series) error {
	serie := &influxClient.Series{
		Name:    prefix + "mem",
		Columns: []string{"free", "used", "actualfree", "actualused", "total"},
		Points:  [][]interface{}{},
	}

	mem := sigar.Mem{}
	if err := mem.Get(); err != nil {
		ch <- nil
		return err
	}
	serie.Points = append(serie.Points, []interface{}{mem.Free, mem.Used, mem.ActualFree, mem.ActualUsed, mem.Total})

	ch <- serie
	return nil
}

func swap(prefix string, ch chan *influxClient.Series) error {
	serie := &influxClient.Series{
		Name:    prefix + "swap",
		Columns: []string{"free", "used", "total"},
		Points:  [][]interface{}{},
	}

	swap := sigar.Swap{}
	if err := swap.Get(); err != nil {
		ch <- nil
		return err
	}
	serie.Points = append(serie.Points, []interface{}{swap.Free, swap.Used, swap.Total})

	ch <- serie
	return nil
}

func uptime(prefix string, ch chan *influxClient.Series) error {
	serie := &influxClient.Series{
		Name:    prefix + "uptime",
		Columns: []string{"length"},
		Points:  [][]interface{}{},
	}

	uptime := sigar.Uptime{}
	if err := uptime.Get(); err != nil {
		ch <- nil
		return err
	}
	serie.Points = append(serie.Points, []interface{}{uptime.Length})

	ch <- serie
	return nil
}

func load(prefix string, ch chan *influxClient.Series) error {
	serie := &influxClient.Series{
		Name:    prefix + "load",
		Columns: []string{"one", "five", "fifteen"},
		Points:  [][]interface{}{},
	}

	load := sigar.LoadAverage{}
	if err := load.Get(); err != nil {
		ch <- nil
		return err
	}
	serie.Points = append(serie.Points, []interface{}{load.One, load.Five, load.Fifteen})

	ch <- serie
	return nil
}

func network(prefix string, ch chan *influxClient.Series) error {
	fi, err := os.Open("/proc/net/dev")
	if err != nil {
		return err
	}
	defer fi.Close()

	serie := &influxClient.Series{
		Name: prefix + "network",
		Columns: []string{"iface",
			"recv_bytes", "recv_packets", "recv_errs",
			"recv_drop", "recv_fifo", "recv_frame",
			"recv_compressed", "recv_multicast",
			"trans_bytes", "trans_packets", "trans_errs",
			"trans_drop", "trans_fifo", "trans_colls",
			"trans_carrier", "trans_compressed"},
		Points: [][]interface{}{},
	}

	// Search interface
	skip := 2
	scanner := bufio.NewScanner(fi)
	for scanner.Scan() {
		// Skip headers
		if skip > 0 {
			skip--
			continue
		}

		line := scanner.Text()
		tmp := strings.Split(line, ":")
		if len(tmp) < 2 {
			ch <- nil
			return nil
		}

		iface := strings.Trim(tmp[0], " ")
		tmp = strings.Fields(tmp[1])

		var points []interface{}
		points = append(points, iface)

		for i := 0; i < len(serie.Columns)-1; i++ {
			if v, err := strconv.Atoi(tmp[i]); err == nil {
				points = append(points, v)
			} else {
				points = append(points, 0)
			}
		}

		serie.Points = append(serie.Points, points)
	}

	ch <- DiffFromLast(serie)
	return nil
}

func disks(prefix string, ch chan *influxClient.Series) error {
	fi, err := os.Open("/proc/diskstats")
	if err != nil {
		return err
	}
	defer fi.Close()

	serie := &influxClient.Series{
		Name: prefix + "disks",
		Columns: []string{"device",
			"read_ios", "read_merges", "read_sectors", "read_ticks",
			"write_ios", "write_merges", "write_sectors", "write_ticks",
			"in_flight", "io_ticks", "time_in_queue"},
		Points: [][]interface{}{},
	}

	// Search device
	scanner := bufio.NewScanner(fi)
	for scanner.Scan() {
		tmp := strings.Fields(scanner.Text())
		if len(tmp) < 14 {
			ch <- nil
			return nil
		}

		var points []interface{}
		points = append(points, tmp[2])

		for i := 0; i < len(serie.Columns)-1; i++ {
			if v, err := strconv.Atoi(tmp[3+i]); err == nil {
				points = append(points, v)
			} else {
				points = append(points, 0)
			}
		}

		serie.Points = append(serie.Points, points)
	}

	ch <- DiffFromLast(serie)
	return nil
}

func mounts(prefix string, ch chan *influxClient.Series) error {
	serie := &influxClient.Series{
		Name:    prefix + "mounts",
		Columns: []string{"mountpoint", "disk", "free", "total"},
		Points:  [][]interface{}{},
	}

	fi, err := os.Open("/proc/mounts")
	if err != nil {
		return err
	}
	defer fi.Close()

	// Exclude virtual & system fstype
	sysfs := []string{"binfmt_misc", "cgroup", "configfs", "debugfs", "devpts", "devtmpfs", "efivarfs", "fusectl", "mqueue", "none", "proc", "rootfs", "securityfs", "sysfs", "rpc_pipefs", "fuse.gvfsd-fuse", "tmpfs"}

	scanner := bufio.NewScanner(fi)
	for scanner.Scan() {
		tmp := strings.Fields(scanner.Text())

		// Some hack needed to remove "none" virtual mountpoints
		if (stringInSlice(tmp[2], sysfs) == false) && (tmp[0] != "none") {
			fs := syscall.Statfs_t{}

			err := syscall.Statfs(tmp[1], &fs)
			if err != nil {
				return err
			}

			freeBytes := fs.Bfree * uint64(fs.Bsize)
			totalBytes := fs.Blocks * uint64(fs.Bsize)

			serie.Points = append(serie.Points, []interface{}{tmp[1], tmp[0], freeBytes, totalBytes})
		}
	}

	ch <- serie
	return nil
}
