package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

type IPInfo struct {
	IP       string
	Hostname string
	City     string
	Region   string
	Country  string
	Loc      string
	Org      string
	Postal   string
	Timezone string
}

func whatsmyip() string {
	IPINFO_URL := "https://ipinfo.io"
	IPINFO_TOKEN := os.Getenv("IPINFO_TOKEN")

	client := &http.Client{}
	req, err := http.NewRequest("GET", IPINFO_URL, nil)
	if err != nil {
		log.Fatalln(err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %v", IPINFO_TOKEN))
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalln(err)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}

	sb := string(body)
	var ip_data IPInfo

	err = json.Unmarshal([]byte(sb), &ip_data)
	if err != nil {
		log.Fatalln(err)
	}

	final := fmt.Sprintf("IP:  %s\nASN: %s", ip_data.IP, ip_data.Org)
	return final
}
