package bankimport

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// sgml is how most Brazilian banks export OFX 1.x: a plain-text header and tags that are never closed.
const sgml = `OFXHEADER:100
DATA:OFXSGML
VERSION:102
CHARSET:1252

<OFX>
<BANKMSGSRSV1><STMTTRNRS><STMTRS>
<BANKTRANLIST>
<STMTTRN>
<TRNTYPE>DEBIT
<DTPOSTED>20261001120000[-3:BRT]
<TRNAMT>-48.90
<FITID>202610010001
<MEMO>COMPRA MERCADO DO BAIRRO
</STMTTRN>
<STMTTRN>
<TRNTYPE>CREDIT
<DTPOSTED>20261005
<TRNAMT>8500.00
<FITID>202610050002
<NAME>SALARIO
<MEMO>CREDITO SALARIO EMPRESA XYZ
</STMTTRN>
<STMTTRN>
<TRNTYPE>DEBIT
<DTPOSTED>20261006000000
<TRNAMT>-12,50
<FITID>202610060003
<NAME>  PADARIA    CENTRAL
</STMTTRN>
</BANKTRANLIST>
</STMTRS></STMTTRNRS></BANKMSGSRSV1>
</OFX>
`

func TestParseOFXSGML(t *testing.T) {
	got, err := ParseOFX(strings.NewReader(sgml))
	if err != nil {
		t.Fatal(err)
	}
	want := []Entry{
		{ExternalID: "202610010001", Date: "2026-10-01", AmountCents: -4890, Description: "COMPRA MERCADO DO BAIRRO"},
		{ExternalID: "202610050002", Date: "2026-10-05", AmountCents: 850000, Description: "CREDITO SALARIO EMPRESA XYZ"}, // MEMO beats NAME
		{ExternalID: "202610060003", Date: "2026-10-06", AmountCents: -1250, Description: "PADARIA CENTRAL"},              // NAME when there is no MEMO; spaces collapsed
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

// xml is OFX 2.x: well-formed XML with closing tags and extra elements we do not care about.
const xml = `<?xml version="1.0" encoding="UTF-8"?>
<?OFX OFXHEADER="200" VERSION="211" SECURITY="NONE" OLDFILEUID="NONE" NEWFILEUID="NONE"?>
<OFX>
  <CREDITCARDMSGSRSV1><CCSTMTTRNRS><CCSTMTRS>
    <BANKTRANLIST>
      <DTSTART>20261001</DTSTART><DTEND>20261031</DTEND>
      <STMTTRN>
        <TRNTYPE>DEBIT</TRNTYPE>
        <DTPOSTED>20261003</DTPOSTED>
        <TRNAMT>-129.90</TRNAMT>
        <FITID>cc-1</FITID>
        <MEMO>Restaurante São João</MEMO>
      </STMTTRN>
    </BANKTRANLIST>
    <LEDGERBAL><BALAMT>-129.90</BALAMT><DTASOF>20261031</DTASOF></LEDGERBAL>
  </CCSTMTRS></CCSTMTTRNRS></CREDITCARDMSGSRSV1>
</OFX>`

func TestParseOFXXMLCreditCard(t *testing.T) {
	got, err := ParseOFX(strings.NewReader(xml))
	if err != nil {
		t.Fatal(err)
	}
	want := []Entry{{ExternalID: "cc-1", Date: "2026-10-03", AmountCents: -12990, Description: "Restaurante São João"}}
	if !slices.Equal(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func TestParseOFXDecodesLatin1(t *testing.T) {
	// "SÃO JOÃO" and "AÇAÍ" as single Latin-1 bytes (0xC3, 0xC7, 0xCD): not valid UTF-8
	file := "OFXHEADER:100\r\nCHARSET:1252\r\n\r\n<OFX><STMTTRN><DTPOSTED>20261002<TRNAMT>-10.00<FITID>l1<MEMO>S\xc3O JO\xc3O AL\xc7A\xcd</STMTTRN></OFX>"
	got, err := ParseOFX(strings.NewReader(file))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Description != "SÃO JOÃO ALÇAÍ" {
		t.Fatalf("got %+v", got)
	}
}

func TestParseOFXKeepsValidUTF8(t *testing.T) {
	file := "<OFX><STMTTRN><DTPOSTED>20261002<TRNAMT>-10.00<FITID>u1<MEMO>Pão de Açúcar</STMTTRN></OFX>"
	got, err := ParseOFX(strings.NewReader(file))
	if err != nil || got[0].Description != "Pão de Açúcar" {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestParseOFXWithoutFITIDGetsAStableUniqueID(t *testing.T) {
	// the bank repeated the same purchase twice on the same day: both must be kept, with different ids
	file := `<OFX><STMTTRN><DTPOSTED>20261002<TRNAMT>-5.00<MEMO>CAFE</STMTTRN>
<STMTTRN><DTPOSTED>20261002<TRNAMT>-5.00<MEMO>CAFE</STMTTRN></OFX>`
	first, err := ParseOFX(strings.NewReader(file))
	if err != nil || len(first) != 2 {
		t.Fatalf("got %+v, %v", first, err)
	}
	if first[0].ExternalID == "" || first[0].ExternalID == first[1].ExternalID {
		t.Fatalf("ids must be set and different: %q %q", first[0].ExternalID, first[1].ExternalID)
	}
	again, _ := ParseOFX(strings.NewReader(file))
	if first[0].ExternalID != again[0].ExternalID || first[1].ExternalID != again[1].ExternalID {
		t.Fatal("ids must be the same every time the same file is parsed, or re-importing would duplicate")
	}
}

func TestParseOFXSkipsZeroAmountLines(t *testing.T) {
	file := `<OFX><STMTTRN><DTPOSTED>20261002<TRNAMT>0.00<FITID>z<MEMO>TARIFA ISENTA</STMTTRN>
<STMTTRN><DTPOSTED>20261002<TRNAMT>-5.00<FITID>a<MEMO>CAFE</STMTTRN></OFX>`
	got, err := ParseOFX(strings.NewReader(file))
	if err != nil || len(got) != 1 || got[0].ExternalID != "a" {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestParseOFXErrors(t *testing.T) {
	cases := map[string]string{
		"not an OFX file":                         "this is just a text file\nwith two lines",
		"empty":                                   "",
		"an OFX with no transactions":             "OFXHEADER:100\n<OFX><BANKTRANLIST></BANKTRANLIST></OFX>",
		"a transaction with an invalid date":      "<OFX><STMTTRN><DTPOSTED>20261340<TRNAMT>-1.00<FITID>x<MEMO>A</STMTTRN></OFX>",
		"a transaction with no date":              "<OFX><STMTTRN><TRNAMT>-1.00<FITID>x<MEMO>A</STMTTRN></OFX>",
		"a transaction with an unreadable amount": "<OFX><STMTTRN><DTPOSTED>20261002<TRNAMT>abc<FITID>x<MEMO>A</STMTTRN></OFX>",
	}
	for name, file := range cases {
		if got, err := ParseOFX(strings.NewReader(file)); err == nil {
			t.Errorf("%s: got %+v, want an error", name, got)
		}
	}
}

func TestParseOFXErrorsSayWhichTransactionIsWrong(t *testing.T) {
	file := `<OFX><STMTTRN><DTPOSTED>20261002<TRNAMT>-1.00<FITID>ok<MEMO>A</STMTTRN>
<STMTTRN><DTPOSTED>20261002<TRNAMT>oops<FITID>bad-one<MEMO>B</STMTTRN></OFX>`
	_, err := ParseOFX(strings.NewReader(file))
	if err == nil || !strings.Contains(err.Error(), "#2") {
		t.Fatalf("the error should point at transaction #2, got %v", err)
	}
}

func TestParseOFXIgnoresAUTF8ByteOrderMark(t *testing.T) {
	file := "\xef\xbb\xbf<OFX><STMTTRN><DTPOSTED>20261002<TRNAMT>-10.00<FITID>b1<MEMO>Pão</STMTTRN></OFX>"
	got, err := ParseOFX(strings.NewReader(file))
	if err != nil || len(got) != 1 || got[0].Description != "Pão" {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestParseOFXRejectsAHugeFile(t *testing.T) {
	huge := "<OFX>" + strings.Repeat("x", MaxFileBytes)
	if _, err := ParseOFX(strings.NewReader(huge)); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("got %v, want ErrTooLarge", err)
	}
}
