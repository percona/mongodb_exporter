package main

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"time"
)

func resolveHost(hostname string) (string, error) {
	addrs, err := net.LookupHost(hostname)
	if err != nil {
		fmt.Println("Error resolving hostname:", err)
		return "", err
	}
	return addrs[0], nil
}

func main() {
	mongos := os.Getenv("MONGOS")
	mongo11 := os.Getenv("MONGO11")
	mongo12 := os.Getenv("MONGO12")
	mongo13 := os.Getenv("MONGO13")

	mongo21 := os.Getenv("MONGO21")
	mongo22 := os.Getenv("MONGO22")
	mongo23 := os.Getenv("MONGO23")

	mongo31 := os.Getenv("MONGO31")
	mongo32 := os.Getenv("MONGO32")
	mongo33 := os.Getenv("MONGO33")

	port := os.Getenv("PORT")
	if port == "" {
		port = "27017"
	}

	fmt.Println("Waiting for startup..")
	for {
		_, err := net.DialTimeout("tcp", net.JoinHostPort(mongos, port), 1*time.Second)
		if err == nil {
			break
		}
		fmt.Print(".")
		time.Sleep(1 * time.Second)
	}
	fmt.Println("Started..")

	fmt.Printf("init-shard.sh time now: %s\n", time.Now().Format("15:04:05"))

	rs1 := os.Getenv("RS1")
	rs2 := os.Getenv("RS2")

	mongodb11Resolved, _ := resolveHost(mongo11)
	mongodb12Resolved, _ := resolveHost(mongo12)
	mongodb13Resolved, _ := resolveHost(mongo13)

	mongodb21Resolved, _ := resolveHost(mongo21)
	mongodb22Resolved, _ := resolveHost(mongo22)
	mongodb23Resolved, _ := resolveHost(mongo23)

	mongodb31Resolved, _ := resolveHost(mongo31)
	mongodb32Resolved, _ := resolveHost(mongo32)
	mongodb33Resolved, _ := resolveHost(mongo33)

	cmd := exec.Command("mongo", "--host", fmt.Sprintf("%s:%s", mongos, port))
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	input := fmt.Sprintf(`
		sh.addShard("%s/%s:%s,%s:%s,%s:%s");
		sh.addShard("%s/%s:%s,%s:%s,%s:%s");
		sh.status();
		quit();
	`, rs1, mongodb11Resolved, port, mongodb12Resolved, port, mongodb13Resolved, port, rs2, mongodb21Resolved, port, mongodb22Resolved, port, mongodb23Resolved, port)

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Start()
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	cmd.Stdin.Write([]byte(input))
	cmd.Wait()
}

