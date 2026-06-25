package gb28181

import "testing"

func TestCatalogDecoder2016Ignores2022Fields(t *testing.T) {
	body := []byte(`<?xml version="1.0" encoding="GB2312"?>
<Response>
<CmdType>Catalog</CmdType>
<SN>1</SN>
<DeviceID>34020000002000000001</DeviceID>
<SumNum>1</SumNum>
<DeviceList Num="1">
<Item>
<DeviceID>34020000001320000001</DeviceID>
<Name>Camera 1</Name>
<Manufacturer>Vendor</Manufacturer>
<Info>
<PTZType>1</PTZType>
<SecurityLevelCode>3</SecurityLevelCode>
<StreamNumberList>1/2</StreamNumberList>
</Info>
</Item>
</DeviceList>
</Response>`)

	_, channels, supports2022, err := catalogDecoder2016{}.Decode("34020000002000000001", body, "3402000000", &Device{})
	if err != nil {
		t.Fatalf("decode catalog: %v", err)
	}
	if supports2022 {
		t.Fatal("2016 decoder should not report 2022 support")
	}
	if len(channels) != 1 {
		t.Fatalf("expected 1 channel, got %d", len(channels))
	}
	if channels[0].SecurityLevelCode != "" || channels[0].StreamNumberList != "" {
		t.Fatalf("2016 decoder populated 2022 fields: %+v", channels[0])
	}
	if channels[0].PTZType != 1 {
		t.Fatalf("expected PTZType from 2016 Info, got %d", channels[0].PTZType)
	}
}

func TestCatalogDecoder2022ReadsExtendedFields(t *testing.T) {
	body := []byte(`<?xml version="1.0" encoding="GB2312"?>
<Response>
<CmdType>Catalog</CmdType>
<SN>1</SN>
<DeviceID>34020000002000000001</DeviceID>
<SumNum>1</SumNum>
<DeviceList Num="1">
<Item>
<DeviceID>34020000001320000001</DeviceID>
<Name>Camera 1</Name>
<Manufacturer>Vendor</Manufacturer>
<Info>
<PTZType>1</PTZType>
<SecurityLevelCode>3</SecurityLevelCode>
<StreamNumberList>1/2</StreamNumberList>
<PoCommonName>Gate</PoCommonName>
<RecordSaveDays>30</RecordSaveDays>
</Info>
</Item>
</DeviceList>
</Response>`)

	_, channels, supports2022, err := catalogDecoder2022{}.Decode("34020000002000000001", body, "3402000000", &Device{})
	if err != nil {
		t.Fatalf("decode catalog: %v", err)
	}
	if !supports2022 {
		t.Fatal("2022 decoder should report 2022 support")
	}
	if len(channels) != 1 {
		t.Fatalf("expected 1 channel, got %d", len(channels))
	}
	ch := channels[0]
	if ch.SecurityLevelCode != "3" || ch.StreamNumberList != "1/2" || ch.PoCommonName != "Gate" || ch.RecordSaveDays != 30 {
		t.Fatalf("2022 fields not decoded: %+v", ch)
	}
}
