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
	IpinfoUrl := "https://ipinfo.io"
	IpinfoToken := os.Getenv("IPINFO_TOKEN")

	client := &http.Client{}
	req, err := http.NewRequest("GET", IpinfoUrl, nil)
	if err != nil {
		log.Fatalln(err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %v", IpinfoToken))
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalln(err)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}

	sb := string(body)
	var ipData IPInfo

	err = json.Unmarshal([]byte(sb), &ipData)
	if err != nil {
		log.Fatalln(err)
	}

	final := fmt.Sprintf("IP:  %s\nASN: %s", ipData.IP, ipData.Org)
	return final
}
