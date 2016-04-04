package ddns

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"
)

var cloudflareAPI = "https://api.cloudflare.com/client/v4"
var externalIPAPI = "http://checkip.amazonaws.com/"

// PUT payload for updating the dns record
type updatePayload struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Content string `json:"content"`
	TTL     int    `json:"ttl"`
}

// json error format from cloudflare
type cloudflareErrors struct {
	Errors []struct {
		Message    string `json:"message"`
		ErrorChain []struct {
			Message string `json:"message"`
		} `json:"error_chain"`
	} `json:"errors"`
}

// WAN IP
func getExternalIP() (string, error) {
	resp, err := http.Get(externalIPAPI)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(bytes.TrimSpace(buf)), nil
}

// DNSUpdater Updates Cloudflare's dns entries
type DNSUpdater struct {
	Domain   string
	ZoneID   string
	Email    string
	APIKey   string
	Record   string
	RecordID string
	tick     time.Duration
}

// NewUpdater creates a new dns updator
func NewUpdater(email, apikey, domain, record string) *DNSUpdater {
	return &DNSUpdater{
		Email:  email,
		APIKey: apikey,
		Domain: domain,
		Record: record,
	}
}

// Update continuously updates the dns record
func (d *DNSUpdater) Update() {
	// default to 24 hours
	if d.tick == 0 {
		d.tick = 24 * time.Hour
	}
	ticker := time.NewTicker(d.tick)

	// run atleast once
	d.updateRecord()

	// update every tick
	for {
		<-ticker.C
		d.updateRecord()
	}
}

// SetTTL allows you to set how long to wait for before refreshing the
// the dns record
func (d *DNSUpdater) SetTTL(t time.Duration) {
	d.tick = t
}

// update ze record
func (d *DNSUpdater) updateRecord() {
	// Make sure we have our record id before doing anything
	if d.RecordID == "" {
		if err := d.updateRecordID(); err != nil {
			log.Printf("error getting record id: %q\n", err)
			return
		}
	}

	// Grab our external IP
	ip, err := getExternalIP()
	if err != nil {
		log.Printf("error: could not get external ip: %q\n", err)
		return
	}

	// Prep the PUT payload
	payload := updatePayload{
		ID:      d.RecordID,
		Name:    d.Record + "." + d.Domain,
		Type:    "A",
		Content: ip,
		TTL:     int(d.tick / time.Second),
	}

	buf := bytes.Buffer{}
	encoder := json.NewEncoder(&buf)
	err = encoder.Encode(&payload)
	if err != nil {
		log.Printf("error: could not encode cloudflare update payload: %q\n", err)
		return
	}

	// Create the http request
	updateRecordURL := fmt.Sprintf("%s/zones/%s/dns_records/%s", cloudflareAPI, d.ZoneID, d.RecordID)
	request, err := http.NewRequest("PUT", updateRecordURL, &buf)
	if err != nil {
		log.Printf("error: could not create new request for updating the dns record: %q\n", err)
		return
	}
	d.setAuthHeaders(request)
	request.Header.Add("Content-Type", "application/json")
	client := http.Client{}
	resp, err := client.Do(request)

	if err != nil {
		log.Printf("error: could not update dns record: %q\n", err)
		return
	}

	// Check to see if we got a success
	if resp.StatusCode == http.StatusOK {
		log.Printf("updated success: record %q, ip %q\n", d.Record+"."+d.Domain, ip)
		return
	}

	// log any errors from cloudflare
	errorsStruct := cloudflareErrors{}
	errorDecoder := json.NewDecoder(resp.Body)
	err = errorDecoder.Decode(&errorsStruct)
	if err != nil {
		log.Printf("error: got non-200 from cloudflare: could not decode error response: %q\n", err)
		return
	}
	for _, errorMsg := range errorsStruct.Errors {
		log.Printf("error from cloudflare: %q\n", errorMsg.Message)
		var errorChain []string
		for _, chain := range errorMsg.ErrorChain {
			errorChain = append(errorChain, chain.Message)
		}
		log.Printf("error chain: %s", strings.Join(errorChain, ","))
	}
}

// utility to set API call auth headers
func (d *DNSUpdater) setAuthHeaders(r *http.Request) {
	r.Header.Add("X-Auth-Email", d.Email)
	r.Header.Add("X-Auth-Key", d.APIKey)
}

// retrieve the zone ID for a domain
func (d *DNSUpdater) updateZoneID() error {
	// Create the http request
	client := http.Client{}
	request, err := http.NewRequest("GET", cloudflareAPI+"/zones?name="+d.Domain, nil)
	if err != nil {
		return err
	}
	d.setAuthHeaders(request)
	resp, err := client.Do(request)
	if err != nil {
		return err
	}

	// Parse the response
	respStruct := struct {
		Result []struct {
			ID string `json:"id"`
		} `json:"result"`
	}{}
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&respStruct)
	if err != nil {
		return err
	}

	// Make sure we actually got some data back from cloudflare
	if len(respStruct.Result) == 0 {
		return errors.New("error: cloudflare did not return any zones")
	}

	// Save our zone
	d.ZoneID = respStruct.Result[0].ID
	return nil
}

// retrieve the record ID for a dns record
func (d *DNSUpdater) updateRecordID() error {
	// make sure we know which zone we're fetching the record id for
	if d.ZoneID == "" {
		if err := d.updateZoneID(); err != nil {
			return err
		}
	}
	// make the http request to get the dns records
	client := http.Client{}
	dnsRecordsURL := fmt.Sprintf("%s/zones/%s/dns_records", cloudflareAPI, d.ZoneID)
	request, err := http.NewRequest("GET", dnsRecordsURL, nil)
	if err != nil {
		return nil
	}
	d.setAuthHeaders(request)
	resp, err := client.Do(request)
	if err != nil {
		return err
	}

	// decode the response
	recordsStruct := struct {
		Result []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"result"`
	}{}
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&recordsStruct)

	if err != nil {
		return err
	}

	// make sure we actually get dns records back from cloudflare
	if len(recordsStruct.Result) == 0 {
		return errors.New("error: cloudflare did not return any dns records")
	}

	// Find the record that we want
	desiredRecord := d.Record + "." + d.Domain
	for _, r := range recordsStruct.Result {
		if r.Name == desiredRecord {
			d.RecordID = r.ID
			break
		}
	}
	if d.RecordID == "" {
		return errors.New("error: could not find the record to update")
	}
	return nil
}
