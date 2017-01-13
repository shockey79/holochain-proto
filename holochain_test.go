package holochain

import (
	"fmt"
	"strconv"
	"testing"
	"time"
	"github.com/google/uuid"
	"os"
	"crypto/sha256"
	"crypto/ecdsa"
	gob "encoding/gob"
	"bytes"
	"math/big"
	toml "github.com/BurntSushi/toml"
	. "github.com/smartystreets/goconvey/convey"
)

func TestNew(t *testing.T) {
	var key ecdsa.PrivateKey
	h := New("Joe",&key,"some/path")
	nID := string(uuid.NodeID());
	if (nID != string(h.Id.NodeID()) ) {
		t.Error("expected holochain UUID NodeID to be "+nID+" got",h.Id.NodeID())
	}
	if (h.Types[0] != "myData") {
		t.Error("data got:",h.Types)
	}
	if (h.agent != "Joe") {
		t.Error("agent got:",h.agent)
	}
	if (h.privKey != &key) {
		t.Error("key got:",h.privKey)
	}
	if (h.path != "some/path") {
		t.Error("path got:",h.path)
	}

}

func TestGenDev(t *testing.T) {
	d,s := setupTestService()
	defer cleanupTestDir(d)
	name := "test"
	root := s.Path+"/"+name
	if _,err := s.IsConfigured(name); err == nil {
		t.Error("expected no dna got:",err)
	}

	h, err := s.Load("test")
	ExpectErrString(t,err,"open "+root+"/"+DNAFileName+": no such file or directory")

	h,err = GenDev(root)
	if err != nil {
		t.Error("expected no error got",err)
	}

	if _,err = s.IsConfigured(name); err != nil {
		t.Error(err)
	}

	lh, err := s.Load(name)
	if  err != nil {
		t.Error("Error parsing loading",err)
	}

	if (lh.Id != h.Id) {
		t.Error("expected matching ids!")
	}

	_,err = GenDev(root)
	ExpectErrString(t,err,"holochain: "+root+" already exists")


}

func TestNewEntry(t *testing.T) {
	d,s := setupTestService()
	defer cleanupTestDir(d)
	n := "test"
	path := s.Path+"/"+n
	h,err := GenDev(path)
	ExpectNoErr(t,err)
	myData := `(message (from "art") (to "eric") (contents "test"))`

	link := NewHash("3vemK25pc5ewYtztPGYAdX39uXuyV13xdouCnZUr8RMA") // dummy link hash

	now := time.Unix(1,1) // pick a constant time so the test will always work

	e := GobEntry{C:myData}
	headerHash,header,err := h.NewEntry(now,"myData",link,&e)
	Convey("parameters passed in should be in the header", t, func() {
		So(err,ShouldBeNil)
		So(header.Time == now,ShouldBeTrue)
		So(header.Type,ShouldEqual,"myData")
		So(header.HeaderLink,ShouldEqual,link)
	})
	Convey("the entry hash is correct", t, func() {
		So(header.EntryLink.String(),ShouldEqual,"G5tGxuTygAMYx2BMagaWJrYpwtiVuDFUtnYkX6rpL1Y5")
	})

	// can't check against a fixed hash because signature created each time test runs is
	// different (though valid) so the header will hash to a different value
	Convey("the returned header hash is the SHA256 of the byte encoded header", t, func() {
		b,_ := ByteEncoder(&header)
		hh := Hash(sha256.Sum256(b))
		So(headerHash,ShouldEqual,hh)
	})

	/*	if a != "EdkgsdwazMZc9vJJgGXgbGwZFvy2Wa1hLCjngmkw3PbF" {
		t.Error("expected EdkgsdwazMZc9vJJgGXgbGwZFvy2Wa1hLCjngmkw3PbF got:",a)
	}*/

	// check the my signature of the entry
	pub,err := UnmarshalPublicKey(s.Path,PubKeyFileName)
	ExpectNoErr(t,err)
	sig := header.MySignature
	hash := header.EntryLink[:]
	if !ecdsa.Verify(pub,hash,sig.R,sig.S) {t.Error("expected verify!")}

	s1 := fmt.Sprintf("%v",*header)
	d1 := fmt.Sprintf("%v",myData)

	h2,_,err := h.Get(headerHash,false)
	ExpectNoErr(t,err)
	s2 := fmt.Sprintf("%v",h2)
	if s2!=s1 {t.Error("expected header to match! \n  "+s2+" \n  "+s1)}

	var d2 interface{}
	h2,d2,err = h.Get(headerHash,true)
	ExpectNoErr(t,err)
	s2 = fmt.Sprintf("%v",d2)
	if s2!=d1 {t.Error("expected entry to match! \n  "+s2+" \n  "+d1)}


}

func TestHeader(t *testing.T) {
	var h1,h2 Header
	h1 = mkTestHeader()

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(&h1)
	ExpectNoErr(t,err)

	dec := gob.NewDecoder(&buf)
	err = dec.Decode(&h2)

	s1 := fmt.Sprintf("%v",h1)
	s2 := fmt.Sprintf("%v",h2)
	if s2!=s1 {t.Error("expected binary match! "+s2+" "+s1)}
}

func TestGob(t *testing.T) {
	gob.Register(Header{})

	g := GobEntry{C:mkTestHeader()}
	v,err := g.Marshal()
	ExpectNoErr(t,err)
	var g2 GobEntry
	err = g2.Unmarshal(v)
	ExpectNoErr(t,err)
	sg1 := fmt.Sprintf("%v",g)
	sg2 := fmt.Sprintf("%v",g)
	if sg2!=sg1 {t.Error("expected gob match! \n  "+sg2+" \n  "+sg1)}
}

func TestJSONEntry(t *testing.T) {
	/* Not yet implemented or used
	g := JSONEntry{C:Config{Port:8888}}
	v,err := g.Marshal()
	ExpectNoErr(t,err)
	var g2 JSONEntry
	err = g2.Unmarshal(v)
	ExpectNoErr(t,err)
	if g2!=g {t.Error("expected JSON match! "+fmt.Sprintf("%v",g)+" "+fmt.Sprintf("%v",g2))}
*/
}

func TestGenChain(t *testing.T) {
	gob.Register(KeyEntry{})
	d,s := setupTestService()
	defer cleanupTestDir(d)
	n := "test"
	path := s.Path+"/"+n
	h,err := GenDev(path)
	ExpectNoErr(t,err)

	Convey("Generating DNA Hashes should re-save the DNA file",t,func() {
		err = h.GenDNAHashes()
		So(err, ShouldBeNil)
		var h2 Holochain
		_,err = toml.DecodeFile(path+"/"+DNAFileName, &h2)
		So(err, ShouldBeNil)
		So( h2.ValidatorHashes["myData"],ShouldEqual, h.ValidatorHashes["myData"] )
	})

	Convey("before GenChain call ID call should fail",t, func() {
		_,err := h.ID()
		So(err.Error(), ShouldEqual, "holochain: chain not started")
	})

	var headerHash Hash
	Convey("GenChain call works",t, func() {

		headerHash,err = h.GenChain()
		So(err, ShouldBeNil)
	})

	var header Header
	Convey("top link should be Key entry",t, func() {
		hdr,entry,err := h.Get(headerHash,true)
		So(err, ShouldBeNil)
		header = hdr
		var k KeyEntry = entry.(KeyEntry)
		So(k.ID,ShouldEqual,h.agent)
		//So(k.Key,ShouldEqual,"something?") // test that key got correctly retrieved
	})

	var dnaHash Hash
	Convey("next link should be the dna entry",t, func() {
		hd,entry,err := h.Get(header.HeaderLink,true)
		So(err, ShouldBeNil)

		var buf bytes.Buffer
		err = h.EncodeDNA(&buf)
		So(string(entry.([]byte)), ShouldEqual, string(buf.Bytes()))
		dnaHash = hd.EntryLink
	})

	Convey("holochain id should have now been set",t, func() {
		id,err := h.ID()
		So(err, ShouldBeNil)
		So(id.String(),ShouldEqual,dnaHash.String())
	})
}

//----------------------------------------------------------------------------------------

func ExpectErrString(t *testing.T,err error,text string) {
	if err.Error() != text {
		t.Error("expected '"+text+"' got",err)
	}
}

func ExpectNoErr(t *testing.T,err error) {
	if err != nil {
		t.Error("expected no err, got",err)
	}
}

func mkTestDirName() string {
	t := time.Now()
	d := "/tmp/holochain_test"+strconv.FormatInt(t.Unix(),10)+"."+strconv.Itoa(t.Nanosecond())
	return d
}

func setupTestService() (d string,s *Service) {
	d = mkTestDirName()
	agent := Agent("Herbert <h@bert.com>")
	s,err := Init(d,agent)
	if err != nil {panic(err)}
	return
}

func setupTestDir() string {
	d := mkTestDirName();
	return d
}

func cleanupTestDir(path string) {
	func() {os.RemoveAll(path)}()
}

func mkTestHeader() Header {
	hl := NewHash("1vemK25pc5ewYtztPGYAdX39uXuyV13xdouCnZUr8RMA")
	el := NewHash("2vemK25pc5ewYtztPGYAdX39uXuyV13xdouCnZUr8RMA")
	now := time.Unix(1,1) // pick a constant time so the test will always work
	h1 := Header{Time:now,Type:"fish",Meta:"dog",
		HeaderLink:hl,
		EntryLink:el,
		MySignature:Signature{R:new(big.Int),S:new(big.Int)},
	}
	h1.MySignature.R.SetUint64(123)
	h1.MySignature.S.SetUint64(321)
	return h1
}
