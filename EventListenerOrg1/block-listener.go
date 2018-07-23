/*
 Copyright IBM Corp All Rights Reserved.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

      http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package main

import (
	"errors"
	"flag"
	"math/big"
	"fmt"
	"encoding/json"
	"os"	
	"crypto/sha256"
	"strings"
	"crypto/rand"
	"github.com/hyperledger/fabric-sdk-go/pkg/fabsdk"
	"github.com/hyperledger/fabric-sdk-go/pkg/client/channel"
	"github.com/hyperledger/fabric-sdk-go/pkg/core/config"
	"github.com/hyperledger/fabric/core/ledger/util"
	"github.com/hyperledger/fabric/events/consumer"
	"github.com/hyperledger/fabric/msp/mgmt"
	"github.com/hyperledger/fabric/msp/mgmt/testtools"
	"github.com/hyperledger/fabric/protos/common"
	pb "github.com/hyperledger/fabric/protos/peer"
	"github.com/hyperledger/fabric/protos/utils"
	"github.com/spf13/viper"
)

type adapter struct {
	notfy chan *pb.Event_Block
}

type Request struct {
	S     string
	R     string
	SHA   string
}

func (r *Request) toString() string {
	valueAsBytes, _ := json.Marshal(*r)
	return string(valueAsBytes)
}

func newRequestFromByte(jsonByte []byte) *Request{
	res := &Request{}
	err := json.Unmarshal(jsonByte, res)
	if( err != nil ){
		return nil
	}
	return res
}

//GetInterestedEvents implements consumer.EventAdapter interface for registering interested events
func (a *adapter) GetInterestedEvents() ([]*pb.Interest, error) {
	return []*pb.Interest{{EventType: pb.EventType_BLOCK}}, nil
}

//Recv implements consumer.EventAdapter interface for receiving events
func (a *adapter) Recv(msg *pb.Event) (bool, error) {
	if o, e := msg.Event.(*pb.Event_Block); e {
		a.notfy <- o
		return true, nil
	}
	return false, fmt.Errorf("Receive unknown type event: %v", msg)
}

//Disconnected implements consumer.EventAdapter interface for disconnecting
func (a *adapter) Disconnected(err error) {
	fmt.Print("Disconnected...exiting\n")
	os.Exit(1)
}

func createEventClient(eventAddress string, _ string) *adapter {
	var obcEHClient *consumer.EventsClient

	done := make(chan *pb.Event_Block)
	adapter := &adapter{notfy: done}
	obcEHClient, _ = consumer.NewEventsClient(eventAddress, 5, adapter)
	if err := obcEHClient.Start(); err != nil {
		fmt.Printf("could not start chat. err: %s\n", err)
		obcEHClient.Stop()
		return nil
	}

	return adapter
}
func getTxPayload(tdata []byte) (*common.Payload, error) {
	if tdata == nil {
		return nil, errors.New("Cannot extract payload from nil transaction")
	}

	if env, err := utils.GetEnvelopeFromBlock(tdata); err != nil {
		return nil, fmt.Errorf("Error getting tx from block(%s)", err)
	} else if env != nil {
		// get the payload from the envelope
		payload, err := utils.GetPayload(env)
		if err != nil {
			return nil, fmt.Errorf("Could not extract payload from envelope, err %s", err)
		}
		return payload, nil
	}
	return nil, nil
}

// getChainCodeEvents parses block events for chaincode events associated with individual transactions
func getChainCodeEvents(tdata []byte) (*pb.ChaincodeEvent, error) {
	if tdata == nil {
		return nil, errors.New("Cannot extract payload from nil transaction")
	}

	if env, err := utils.GetEnvelopeFromBlock(tdata); err != nil {
		return nil, fmt.Errorf("Error getting tx from block(%s)", err)
	} else if env != nil {
		// get the payload from the envelope
		payload, err := utils.GetPayload(env)
		if err != nil {
			return nil, fmt.Errorf("Could not extract payload from envelope, err %s", err)
		}

		chdr, err := utils.UnmarshalChannelHeader(payload.Header.ChannelHeader)
		if err != nil {
			return nil, fmt.Errorf("Could not extract channel header from envelope, err %s", err)
		}

		if common.HeaderType(chdr.Type) == common.HeaderType_ENDORSER_TRANSACTION {
			tx, err := utils.GetTransaction(payload.Data)
			if err != nil {
				return nil, fmt.Errorf("Error unmarshalling transaction payload for block event: %s", err)
			}
			chaincodeActionPayload, err := utils.GetChaincodeActionPayload(tx.Actions[0].Payload)
			if err != nil {
				return nil, fmt.Errorf("Error unmarshalling transaction action payload for block event: %s", err)
			}
			propRespPayload, err := utils.GetProposalResponsePayload(chaincodeActionPayload.Action.ProposalResponsePayload)
			if err != nil {
				return nil, fmt.Errorf("Error unmarshalling proposal response payload for block event: %s", err)
			}
			caPayload, err := utils.GetChaincodeAction(propRespPayload.Extension)
			if err != nil {
				return nil, fmt.Errorf("Error unmarshalling chaincode action for block event: %s", err)
			}
			ccEvent, err := utils.GetChaincodeEvents(caPayload.Events)

			if ccEvent != nil {
				return ccEvent, nil
			}
		}
	}
	return nil, errors.New("No events found")
}

const (
	eventoSHA      = "eventRand"
	eventoValori   = "eventValue"
	channelID      = "mychannel"
	orgName        = "Org1"
	orgAdmin       = "Admin"
	ordererOrgName = "Orderer"
	ccID           = "ccOperations7"
	ccInvoke       = "ccEvent"
	pathMSP        = "/home/blockchain/Desktop/Code/Network/crypto-config/peerOrganizations/org1.example.com/users/Admin@org1.example.com/msp"
	ip             = "0.0.0.0:7053"
)

func main() {
	// For environment variables
	viper.SetEnvPrefix("core")
	viper.AutomaticEnv()
	replacer := strings.NewReplacer(".", "_")
	viper.SetEnvKeyReplacer(replacer)
	bits := 20
	var eventAddress string
	var chaincodeID string
	var mspDir string
	var mspId string
	var m map[string]*Request
	m = make(map[string]*Request)	
	flag.StringVar(&eventAddress, "events-address", ip, "address of events server")
	flag.StringVar(&chaincodeID, "eventValue", ccID, "listen to events from given chaincode")
	flag.StringVar(&mspDir, "events-mspdir", pathMSP, "set up the msp direction")
	flag.StringVar(&mspId, "events-mspid", orgName+"MSP", "set up the mspid")
	flag.Parse()
	configPath := "file/config.yaml"
	configOpt := config.FromFile(configPath)
	sdk , err := fabsdk.New(configOpt)
	if err != nil {
		fmt.Println("Failed to create new SDK: %s", err)
	}
	defer sdk.Close()
	org1ChannelClientContext := sdk.ChannelContext(channelID, fabsdk.WithUser(orgAdmin), fabsdk.WithOrg(orgName))
	channelClient, err := channel.New(org1ChannelClientContext)
	if err != nil {
		fmt.Printf("Failed to create new channel client: %s\n", err)
	}
	//if no msp info provided, we use the default MSP under fabric/sampleconfig
	if mspDir == "" {
		err := msptesttools.LoadMSPSetupForTesting()
		if err != nil {
			fmt.Printf("Could not initialize msp, err: %s\n", err)
			os.Exit(-1)
		}
	} else {
		//load msp info
		err := mgmt.LoadLocalMsp(mspDir, nil, mspId)
		if err != nil {
			fmt.Printf("Could not initialize msp, err: %s\n", err)
			os.Exit(-1)
		}
	}

	fmt.Printf("Event Address: %s\n", eventAddress)

	a := createEventClient(eventAddress, chaincodeID)
	if a == nil {
		fmt.Println("Error creating event client")
		return
	}

	for {
		select {
		case b := <-a.notfy:
			fmt.Println("")
			fmt.Println("")
			fmt.Println("Received block")
			fmt.Println("--------------")
			txsFltr := util.TxValidationFlags(b.Block.Metadata.Metadata[common.BlockMetadataIndex_TRANSACTIONS_FILTER])
			for i, r := range b.Block.Data.Data {
				tx, _ := getTxPayload(r)
				if tx != nil {
					chdr, err := utils.UnmarshalChannelHeader(tx.Header.ChannelHeader)
					if err != nil {
						fmt.Print("Error extracting channel header\n")
						return
					}
					if txsFltr.IsInvalid(i) {
						fmt.Println("")
						fmt.Println("")
						fmt.Printf("Received invalid transaction from channel '%s'\n", chdr.ChannelId)
						fmt.Println("--------------")
						fmt.Printf("Transaction invalid: TxID: %s\n", chdr.TxId)
					} else {
						fmt.Printf("Received transaction from channel '%s'\n", chdr.ChannelId)	
						if event, err := getChainCodeEvents(r); err == nil {
							if len(chaincodeID) != 0 && event.ChaincodeId == chaincodeID {
								switch event.EventName {
									case eventoSHA:
										r, _ := rand.Prime(rand.Reader, bits)
										s, _ := rand.Prime(rand.Reader, bits)
										c := sha256.Sum256([]byte(s.String() + r.String()))
fmt.Printf("concatenazione",r.String() + s.String())
									c_Int := big.NewInt(0)
										c_Int.SetBytes(c[:])
										req := &Request{r.String(), s.String(), c_Int.String()}										
										m[string(event.Payload)]=req
										fmt.Println("")
										fmt.Println("")
										fmt.Printf("Received chaincode event from channel '%s'\n", chdr.ChannelId)
										fmt.Println("-------------------------------------------------")
										fmt.Printf("Chaincode Event:%+v\n", event)
										res, err := channelClient.Execute(channel.Request{
												ChaincodeID: ccInvoke,
												Fcn:         "setRandom",
												Args: [][]byte{
													[]byte(c_Int.String()),
													event.Payload,
												},
											})
										if err != nil {
											fmt.Println("Execution failed: %s\n",err)
										}else{
											fmt.Println("Execution completed:%s\n",res.ChaincodeStatus)								
										}
									case eventoValori:
										value, ok := m[string(event.Payload)]
										if(!ok){
											fmt.Println("Cannot find id request")										
										}
										fmt.Printf("sha:%s",value.toString())
										fmt.Println("")
										fmt.Println("")
										fmt.Printf("Received chaincode event from channel '%s'\n", chdr.ChannelId)
										fmt.Println("-------------------------------------------------")
										fmt.Printf("Chaincode Event:%+v\n", event)
										res, err := channelClient.Execute(channel.Request{
												ChaincodeID: ccInvoke,
												Fcn:         "setRealValue",
												Args: [][]byte{
													[]byte(value.S),
													[]byte(value.R),
													event.Payload,
												},
											})
										if err != nil {
											fmt.Println("Execution failed: %s\n",err)
										}else{
											fmt.Printf("Execution completed:%s %s\n",res.ChaincodeStatus,string(res.Payload))								
										}


								}		
							}
						}
					}
				}
			}
		}
	}
}
