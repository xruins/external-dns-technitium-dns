// Package technitium provides a client for the Technitium DNS Server REST API.
package technitium

import "encoding/json"

// apiResponse is the generic envelope returned by every Technitium API call.
type apiResponse struct {
	Status       string          `json:"status"`
	ErrorMessage string          `json:"errorMessage,omitempty"`
	Response     json.RawMessage `json:"response,omitempty"`
}

// loginResponse holds the authentication token returned after a successful login.
type loginResponse struct {
	Token string `json:"token"`
}

// Record represents a single DNS record as returned by the Technitium API.
type Record struct {
	Name     string          `json:"name"`
	Type     string          `json:"type"`
	TTL      int64           `json:"ttl"`
	Disabled bool            `json:"disabled"`
	Comments string          `json:"comments"`
	RData    json.RawMessage `json:"rData"`
}

// getRecordsResponse holds the zone info and record list from GET /api/zones/records/get.
type getRecordsResponse struct {
	Records []Record `json:"records"`
}

// rDataA holds the payload of an A or AAAA record.
type rDataA struct {
	IPAddress string `json:"ipAddress"`
}

// rDataCNAME holds the payload of a CNAME record.
type rDataCNAME struct {
	CName string `json:"cname"`
}

// rDataTXT holds the payload of a TXT record.
type rDataTXT struct {
	Text string `json:"text"`
}

// rDataMX holds the payload of an MX record.
type rDataMX struct {
	Exchange   string `json:"exchange"`
	Preference uint16 `json:"preference"`
}

// rDataNS holds the payload of an NS record.
type rDataNS struct {
	NameServer string `json:"nameServer"`
}

// rDataSRV holds the payload of an SRV record.
type rDataSRV struct {
	Priority uint16 `json:"priority"`
	Weight   uint16 `json:"weight"`
	Port     uint16 `json:"port"`
	Target   string `json:"target"`
}

// rDataCAA holds the payload of a CAA record.
type rDataCAA struct {
	Flags uint8  `json:"flags"`
	Tag   string `json:"tag"`
	Value string `json:"value"`
}

// rDataANAME holds the payload of a Technitium-specific ANAME record.
type rDataANAME struct {
	AName string `json:"aname"`
}
