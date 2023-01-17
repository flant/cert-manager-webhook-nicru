package main

func (c *NicruClient) Txt(recordName string) {
	var record = &Request{
		RrList: &RrList{
			Rr: []Rr{},
		},
	}
	record.RrList.Rr = append(record.RrList.Rr, Rr{
		Name: recordName,
		Type: "TXT",
		Txt: &TxtRecord{
			String: "Test record"}})
	c.createTxt(record)
}
