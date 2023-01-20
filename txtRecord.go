package main

func (c *NicruClient) Txt(recordName, serviceName, zoneName, content string) string {
	var record = &Request{
		RrList: &RrList{
			Rr: []*Rr{},
		},
	}
	record.RrList.Rr = append(record.RrList.Rr, &Rr{
		Name: recordName,
		Type: "TXT",
		Txt: &TxtRecord{
			String: content}})
	rrId := c.createRecord(record, serviceName, zoneName)
	return rrId
}
