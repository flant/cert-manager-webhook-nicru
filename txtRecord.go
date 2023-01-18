package main

func (c *NicruClient) Txt(recordName, serviceName, zoneName string) string {
	var record = &Request{
		RrList: &RrList{
			Rr: []*Rr{},
		},
	}
	record.RrList.Rr = append(record.RrList.Rr, &Rr{
		Name: recordName,
		Type: "TXT",
		Txt: &TxtRecord{
			String: "Test record"}})
	rrId := c.createRecord(record, serviceName, zoneName)
	return rrId
}
