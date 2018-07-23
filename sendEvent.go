package main

import (
	"fmt"
	"math/big"
	"crypto/sha256"
	"encoding/json"
	"github.com/hyperledger/fabric/core/chaincode/shim"
	"github.com/hyperledger/fabric/protos/peer"
	"github.com/hyperledger/fabric/protos/msp"
	"crypto/x509"
	"encoding/pem"
	"github.com/golang/protobuf/proto"
	"strings"
)
const(
	numOrgs      = 2
	valoriReali  = "valoriReali"
	stringaSHA   = "stringaSHA"
)
type Msg struct {
	Value string
	Org   string
}
type Payload struct{
	Progress string
	Info string
}
type Random struct {
	S string
	R   string
	Org   string
}

func (r *Random) toString() string {
	valueAsBytes, _ := json.Marshal(*r)
	return string(valueAsBytes)
}

func (p *Payload) toString() string {
	valueAsBytes, _ := json.Marshal(*p)
	return string(valueAsBytes)
}

func (m *Msg) toString() string {
	valueAsBytes, _ := json.Marshal(*m)
	return string(valueAsBytes)
}

func newRandomFromByte(jsonByte []byte) *Random{
	res := &Random{}
	err := json.Unmarshal(jsonByte, res)
	if( err != nil ){
		return nil
	}
	return res
}



func newMsgFromByte(jsonByte []byte) *Msg{
	res := &Msg{}
	err := json.Unmarshal(jsonByte, res)
	if( err != nil ){
		return nil
	}
	return res
}

func getOrgIn(stub shim.ChaincodeStubInterface, org,stringaSettata string) (bool,error) {
	
	history, errHist := stub.GetHistoryForKey(stringaSettata)
	if errHist != nil {
		return true, fmt.Errorf("Failed to retrive history")
	}
	
	for history.HasNext() {
		kM, err := history.Next()
		if err != nil {
			return true,nil
		}
		orgIn := newMsgFromByte(kM.GetValue()).Org
		if(strings.Compare(orgIn,org) == 0){
			return false,nil
		}
	}
	return true,nil
}


func getPeerOrg(stub shim.ChaincodeStubInterface) (string,error) {
	 serializedID, _ := stub.GetCreator()

	sId := &msp.SerializedIdentity{}
	err := proto.Unmarshal(serializedID, sId)
	if err != nil {
		return "",fmt.Errorf(fmt.Sprintf("Could not deserialize a SerializedIdentity, err %s", err))
	}

	bl, _ := pem.Decode(sId.IdBytes)
	if bl == nil {
		return "",fmt.Errorf(fmt.Sprintf("Failed to decode PEM structure"))
	}
	cert, err := x509.ParseCertificate(bl.Bytes)
	if err != nil {
		return "",fmt.Errorf(fmt.Sprintf("Unable to parse certificate %s", err))
	}
	fmt.Printf("%s",string(cert.Issuer.Organization[0]))
	return string(cert.Issuer.Organization[0]),nil
}

func getPeers(stub shim.ChaincodeStubInterface,asset string) (bool,int,error){
	var err error
	var i int

	i=0
	history, err := stub.GetHistoryForKey(asset)

	for history.HasNext() {
		_, err = history.Next()
		if err != nil {
			return true,0,fmt.Errorf("Failed to retrive history")
		}
		i+=1
	}
	fmt.Printf("\n%d==%d\n",i,numOrgs)
	return i==numOrgs,i,nil
}

func computeRandom(stub shim.ChaincodeStubInterface,realValue string) (string,error){
	history, err := stub.GetHistoryForKey(realValue)
	if err != nil {
		return "", fmt.Errorf("Failed to retrive history")
	}

	kM, err := history.Next()
	if err != nil {
		return "", fmt.Errorf("Failed to retrive an element of history")
	}
	msg := newRandomFromByte(kM.GetValue())
	cXor:=new(big.Int)
	cXor.SetString(msg.R,10)
	for history.HasNext() {
		kM, err = history.Next()

		if err != nil {
			return "", fmt.Errorf("Failed to retrive history")
		}
		msg := newRandomFromByte(kM.GetValue())
		elem:=new(big.Int)
		cXor.SetString(msg.R,10)
		cXor.Xor(cXor,elem)
	}
	
	return cXor.String(), nil
}

func controllaStringa(stub shim.ChaincodeStubInterface,s,r,org,stringaSettata string)(bool,error){
	history, errHist := stub.GetHistoryForKey(stringaSettata)
	if errHist != nil {
		return false, fmt.Errorf("Failed to retrive history")
	}
	
	for history.HasNext() {
		kM, err := history.Next()
		if err != nil {
			return false,nil
		}
		orgMsg :=newMsgFromByte(kM.GetValue())
		orgIn :=orgMsg.Org 
		if(strings.Compare(orgIn,org) == 0){
			c := sha256.Sum256([]byte(r + s))
			c_Int := big.NewInt(0)
			c_Int.SetBytes(c[:])
			_ = stub.PutState("debugging", []byte(c_Int.String()))
			_ = stub.PutState("debugging1", []byte(orgMsg.Value))
			_ = stub.PutState("debuggingR", []byte(r))
			_ = stub.PutState("debuggingS", []byte(s))
			_ = stub.PutState("debuggingCon", []byte(r+s))
			return (strings.Compare(c_Int.String(),orgMsg.Value) == 0),nil
		}
	}
	return false,nil
}

// ******************Function chaincode****************************
type SimpleAsset struct {}



func (t *SimpleAsset) getHistory(stub shim.ChaincodeStubInterface, args []string) (string, error) {
	var str string
	var i int
	var err error

	if len(args) != 1 {
		return "", fmt.Errorf("Incorrect arguments.\n")
	}

	
	history, err := stub.GetHistoryForKey(args[0])
	if err != nil {
		return "", fmt.Errorf("Failed to retrive history")
	}

	str = fmt.Sprintf("-------------------------\n ")

	kM, err := history.Next()
	if err != nil {
		return "", fmt.Errorf("Failed to retrive an element of history")
	}

	i = 0
	
	str += fmt.Sprintf("%d) --> %s \n ", i, string(kM.GetValue()))
	i += 1
	for history.HasNext() {
		kM, err = history.Next()

		if err != nil {
			return "", fmt.Errorf("Failed to retrive history")
		}
		str = str + fmt.Sprintf("%d ) --> %s \n ", i, string(kM.GetValue()))
		i += 1
	}
	str += fmt.Sprintf("-------------------------\n")
	return str, nil
}


func (t *SimpleAsset) get(stub shim.ChaincodeStubInterface, args []string) (string, error) {

	if len(args) != 1 {
		return "", fmt.Errorf("Incorrect arguments. Expecting type message")
	}
	
	value, err := stub.GetState(args[0])
	if ((err != nil) || (len(value) == 0)) {
		return "", fmt.Errorf("Failed to get asset: '%s'", args[0])
	}
	
	return string(value), nil
}

func (t *SimpleAsset) sendEvent(stub shim.ChaincodeStubInterface, args []string) (string, error) {
	
	if(len(args)!=1){
		return "",fmt.Errorf("Expected 1 argument")
	}
	payload:=&Payload{}
	value, err := stub.GetState("Stato"+args[0])
	if ((err != nil) || (len(value) == 0)) {		
		payload.Info="0"
		payload.Progress="0"
		_ = stub.PutState("Stato"+args[0], []byte("0"))
		return payload.toString(),nil
	}
	switch string(value) {
		case "0":
			allPeersIn,prog,errPeer := getPeers(stub,stringaSHA+args[0])
			payload.Progress=fmt.Sprintf("%3.1f",float64(100)*float64(prog/numOrgs))
			if errPeer != nil {
				_ = stub.PutState("Stato"+args[0], []byte(""))
				return "",fmt.Errorf("Something wrong:%s",errPeer)
			}
			if(allPeersIn){
				payload.Info="1"
				_ = stub.PutState("Stato"+args[0], []byte("1"))
				return payload.toString(),nil
			}
			payload.Info="0"
			return payload.toString(),nil
		case "1":
				allPeersIn,prog,errorPeer := getPeers(stub,valoriReali+args[0])
				payload.Progress=fmt.Sprintf("%3.1f",float64(100)*float64(prog/numOrgs))
				if errorPeer != nil {
					_ = stub.PutState("Stato"+args[0], []byte(""))
					return "",fmt.Errorf("Something wrong:%s",errorPeer)
				}
				if(allPeersIn){
						
					defString,errString := computeRandom(stub,valoriReali+args[0])
					payload.Progress=defString
					payload.Info="2"
					fmt.Printf("stringa computata:%s\n",defString)
					if errString != nil {
						_ = stub.PutState("Stato"+args[0], []byte(""))
						return "",fmt.Errorf("Something wrong")
					}
					return payload.toString(),nil
				}
				payload.Info="1"
				return payload.toString(),nil
	}
	return "",nil
	
	
	
}

func (t *SimpleAsset) sendRealValue(stub shim.ChaincodeStubInterface, args []string) (string, error){
	if len(args) != 3 {
		return "", fmt.Errorf("Incorrect arguments. Expecting s, r and id req")
	}
	org,err := getPeerOrg(stub)
	if err != nil {
		return "",fmt.Errorf(fmt.Sprintf("Cannot get Organization:%s", err))
	}
	situation,err := controllaStringa(stub,args[0],args[1],org,stringaSHA+args[2])
	if err != nil {
		return "",fmt.Errorf(fmt.Sprintf("Cannot verify strings:%s", err))
	}
	if(situation){
		post := &Random{args[0],args[1],org}
		_ = stub.PutState(valoriReali+args[2], []byte(post.toString()))
		return "Value setted",nil
	}
	return "Value not setted",nil
	
}

func (t *SimpleAsset) setRandom(stub shim.ChaincodeStubInterface, args []string) (string, error){
	if len(args) != 2 {
		return "", fmt.Errorf("Incorrect arguments. Expecting a random string and id req")
	}
	org,err := getPeerOrg(stub)
	_ = stub.PutState("organizzazione", []byte(org))
	if err != nil {
		return "",fmt.Errorf(fmt.Sprintf("Cannot get Organization:%s", err))
	}
	orgIn,errOrg := getOrgIn(stub,org,stringaSHA+args[1])
	if errOrg != nil {
		return "",fmt.Errorf(fmt.Sprintf("Cannot get result:%s", errOrg))
	}
	if(orgIn){
		post := &Msg{args[0],org}
		_ = stub.PutState(stringaSHA+args[1], []byte(post.toString()))
		return "Value setted",nil
	}
	return fmt.Sprintf("Value not setted. %s has already setted a random string",org),nil	
}



func (t *SimpleAsset) Invoke(stub shim.ChaincodeStubInterface) peer.Response {
	fn, args := stub.GetFunctionAndParameters()

	var result string
	var err error

	switch fn {
	case "sendEvent":
		result, err = t.sendEvent(stub, args)
	case "setRealValue":
		result, err = t.sendRealValue(stub, args)
	case "get":
		result, err = t.get(stub, args)
	case "setRandom":
		result, err = t.setRandom(stub, args)
	case "getHistory":
		result, err = t.getHistory(stub, args)
	}
	if err != nil {
		return shim.Error(err.Error())
	}

	return shim.Success([]byte(result))
}

func (t *SimpleAsset) Init(stub shim.ChaincodeStubInterface) peer.Response {
	
	return shim.Success(nil) 
}



func main() {
	if err := shim.Start(new(SimpleAsset)); err != nil {
		fmt.Printf("Error starting SimpleAsset chaincode: %s\n", err)
	}
}

