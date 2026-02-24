package nicru

import (
	"encoding/xml"
	"strings"
)

type RawXMLElement struct {
	XMLName xml.Name
	Inner   string `xml:",innerxml"`
}

type RecordRequest struct {
	XMLName xml.Name `xml:"request"`
	RrList  RrList   `xml:"rr-list"`
}

type RrList struct {
	Rr []Rr `xml:"rr"`
}

type Rr struct {
	ID      string          `xml:"id,attr,omitempty"`
	Name    string          `xml:"name"`
	IdnName string          `xml:"idn-name,omitempty"`
	TTL     int             `xml:"ttl,omitempty"`
	Type    string          `xml:"type"`
	Txt     *TxtRecord      `xml:"txt,omitempty"`
	Extra   []RawXMLElement `xml:",any"`
}

type TxtRecord struct {
	Strings []string        `xml:"string"`
	Extra   []RawXMLElement `xml:",any"`
}

type APIResponse struct {
	XMLName xml.Name        `xml:"response"`
	Status  string          `xml:"status"`
	Errors  APIErrors       `xml:"errors"`
	Data    APIData         `xml:"data"`
	Extra   []RawXMLElement `xml:",any"`
}

func (r *APIResponse) FormatErrors() string {
	if len(r.Errors.Items) == 0 {
		return "status=" + r.Status + " (no error details)"
	}
	msgs := make([]string, 0, len(r.Errors.Items))
	for _, e := range r.Errors.Items {
		msgs = append(msgs, "["+e.Code+"] "+e.Message)
	}
	return strings.Join(msgs, "; ")
}

type APIErrors struct {
	Items []APIError      `xml:"error"`
	Extra []RawXMLElement `xml:",any"`
}

type APIError struct {
	Code    string `xml:"code,attr"`
	Message string `xml:",chardata"`
}

type APIData struct {
	Zones []ZoneData      `xml:"zone"`
	Extra []RawXMLElement `xml:",any"`
}

type ZoneData struct {
	Name    string          `xml:"name,attr"`
	Service string          `xml:"service,attr"`
	Records []Rr            `xml:"rr"`
	Extra   []RawXMLElement `xml:",any"`
}

type TokenResponse struct {
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
}

func extraFieldNames(extras []RawXMLElement) []string {
	if len(extras) == 0 {
		return nil
	}
	names := make([]string, len(extras))
	for i, e := range extras {
		names[i] = e.XMLName.Local
	}
	return names
}
