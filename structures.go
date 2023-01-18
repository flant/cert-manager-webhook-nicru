package main

import "encoding/xml"

type NicruClient struct {
	token string
}

type Request struct {
	XMLName xml.Name `xml:"request"`
	Text    string   `xml:",chardata"`
	RrList  *RrList  `xml:"rr-list"`
}

type RrList struct {
	Text string `xml:",chardata"`
	Rr   []*Rr  `xml:"rr"`
}

type Rr struct {
	Text string     `xml:",chardata"`
	ID   string     `xml:"id,attr,omitempty"`
	Name string     `xml:"name"`
	Type string     `xml:"type"`
	Txt  *TxtRecord `xml:"txt"`
}

type TxtRecord struct {
	Text   string `xml:",chardata" json:"text,omitempty"`
	String string `xml:"string" json:"string,omitempty"`
}

type Response struct {
	XMLName xml.Name `xml:"response"`
	Text    string   `xml:",chardata"`
	Status  string   `xml:"status"`
	Errors  struct {
		Text  string `xml:",chardata"`
		Error struct {
			Text string `xml:",chardata"`
			Code string `xml:"code,attr"`
		} `xml:"error"`
	} `xml:"errors"`
}

type Zone struct {
	XMLName xml.Name `xml:"response"`
	Text    string   `xml:",chardata"`
	Status  string   `xml:"status"`
	Data    struct {
		Text string `xml:",chardata"`
		Zone []struct {
			Text       string `xml:",chardata"`
			Admin      string `xml:"admin,attr"`
			Enable     string `xml:"enable,attr"`
			HasChanges string `xml:"has-changes,attr"`
			HasPrimary string `xml:"has-primary,attr"`
			ID         string `xml:"id,attr"`
			IdnName    string `xml:"idn-name,attr"`
			Name       string `xml:"name,attr"`
			Payer      string `xml:"payer,attr"`
			Service    string `xml:"service,attr"`
			RR         []*Rr  `xml:"rr"`
		} `xml:"zone"`
	} `xml:"data"`
}
