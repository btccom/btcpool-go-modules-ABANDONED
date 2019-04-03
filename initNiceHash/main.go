package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/samuel/go-zookeeper/zk"
)

type Algorithm struct {
	Name    string `json:"name"`
	MinDiff string `json:"min_diff_working"`
}

type Configuration struct {
	Algorithms []Algorithm
}

type Reply struct {
	Result Configuration `json:"result"`
}

func getNiceHashConfiguration(url string) Configuration {
	log.Printf("Calling NiceHash API %s", url)
	resp, err := http.Get(url)
	if err != nil {
		log.Fatalf("Failed to call NiceHash API: %v", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Failed to read NiceHash API response: %v", err)
	}
	log.Printf("NiceHash API response: %s", body)

	var reply Reply
	err = json.Unmarshal(body, &reply)
	if err != nil {
		log.Fatalf("Failed to unmarshal NiceHash API response: %v", err)
	}

	return reply.Result
}

func populateNiceHashNodes(zookeeper string, path string, config Configuration) {
	servers := strings.Split(zookeeper, ",")
	if len(zookeeper) == 0 || len(servers) == 0 {
		log.Print("ZooKeeper servers are not specificed, exit now")
		return
	}
	c, _, err := zk.Connect(servers, time.Second*5)
	if err != nil {
		log.Fatalf("Failed to connect to ZooKeeper: %v", err)
	}
	defer c.Close()

	dirs := strings.Split(path, "/")
	prefix := ""
	for _, dir := range dirs {
		if len(dir) != 0 {
			prefix += "/" + strings.ToLower(dir)
			exists, _, err := c.Exists(prefix)
			if err != nil {
				log.Fatalf("Failed to check Zookeeper node %s: %v", prefix, err)
			}

			if !exists {
				_, err = c.Create(prefix, []byte{}, 0, zk.WorldACL(zk.PermAll))

				if err != nil {
					log.Fatalf("Failed to create Zookeeper node %s: %v", prefix, err)
				}
			}
		}
	}

	for _, algo := range config.Algorithms {
		minDiff, err := strconv.ParseFloat(algo.MinDiff, 64)
		if err != nil {
			log.Printf("Minimal required difficulty for algorithm %s is not a number: %s", algo.Name, algo.MinDiff)
			continue
		} else {
			log.Printf("Minimal required difficulty for algorithm %s is %s", algo.Name, algo.MinDiff)
		}

		algoName := strings.ToLower(algo.Name)
		if algoName == "daggerhashimoto" {
			minDiff *= 4294967296
		}

		nodeAlgo := prefix + "/" + algoName
		exists, _, err := c.Exists(nodeAlgo)
		if err != nil {
			log.Fatalf("Failed to check ZooKeeper node %s: %v", nodeAlgo, err)
		}
		if !exists {
			_, err := c.Create(nodeAlgo, []byte{}, 0, zk.WorldACL(zk.PermAll))
			if err != nil {
				log.Fatalf("Failed to create ZooKeeper node %s: %v", nodeAlgo, err)
			}
		}

		nodeMinDiff := nodeAlgo + "/min_difficulty"
		exists, _, err = c.Exists(nodeMinDiff)
		data := []byte(strconv.FormatUint(uint64(minDiff), 10))
		if exists {
			_, err := c.Set(nodeMinDiff, data, -1)
			if err != nil {
				log.Fatalf("Failed to write ZooKeeper node %s: %v", nodeMinDiff, err)
			}
		} else {
			_, err := c.Create(nodeMinDiff, data, 0, zk.WorldACL(zk.PermAll))
			if err != nil {
				log.Fatalf("Failed to create ZooKeeper node %s: %v", nodeMinDiff, err)
			}
		}
	}
}

func main() {
	url := flag.String("url", "https://api.nicehash.com/api?method=buy.info", "NiceHash API URL")
	zookeeper := flag.String("zookeeper", "", "ZooKeeper servers separated by comma")
	path := flag.String("path", "/nicehash", "ZooKeeper path to store NiceHash configurations")
	flag.Parse()

	populateNiceHashNodes(*zookeeper, *path, getNiceHashConfiguration(*url))
}
