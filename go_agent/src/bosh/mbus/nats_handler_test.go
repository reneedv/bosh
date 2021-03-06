package mbus_test

import (
	boshhandler "bosh/handler"
	boshlog "bosh/logger"
	. "bosh/mbus"
	boshsettings "bosh/settings"
	fakesettings "bosh/settings/fakes"
	"encoding/json"
	"github.com/cloudfoundry/yagnats"
	"github.com/cloudfoundry/yagnats/fakeyagnats"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNatsHandlerStart(t *testing.T) {
	settings := &fakesettings.FakeSettingsService{
		AgentId: "my-agent-id",
		MbusUrl: "nats://foo:bar@127.0.0.1:1234",
	}
	client, handler := buildNatsClientAndHandler(settings)

	var receivedRequest boshhandler.Request

	handler.Start(func(req boshhandler.Request) (resp boshhandler.Response) {
		receivedRequest = req
		return boshhandler.NewValueResponse("expected value")
	})
	defer handler.Stop()

	// check connection
	assert.NotNil(t, client.ConnectedConnectionProvider)

	// check subscriptions
	assert.Equal(t, len(client.Subscriptions), 1)
	subscriptions := client.Subscriptions["agent.my-agent-id"]
	assert.Equal(t, len(subscriptions), 1)

	// test subscription callback
	expectedPayload := []byte(`{"method":"ping","arguments":["foo","bar"], "reply_to": "reply to me!"}`)
	subscription := client.Subscriptions["agent.my-agent-id"][0]
	subscription.Callback(&yagnats.Message{
		Subject: "agent.my-agent-id",
		Payload: expectedPayload,
	})

	// request received
	assert.Equal(t, receivedRequest, boshhandler.Request{
		ReplyTo: "reply to me!",
		Method:  "ping",
		Payload: expectedPayload,
	})

	// response sent
	assert.Equal(t, len(client.PublishedMessages), 1)
	messages := client.PublishedMessages["reply to me!"]

	assert.Equal(t, len(messages), 1)
	message := messages[0]

	assert.Equal(t, []byte(`{"value":"expected value"}`), message.Payload)
}

func TestNatsSendPeriodicHeartbeat(t *testing.T) {
	settings := &fakesettings.FakeSettingsService{
		AgentId: "my-agent-id",
		MbusUrl: "nats://foo:bar@127.0.0.1:1234",
	}
	client, handler := buildNatsClientAndHandler(settings)

	errChan := make(chan error, 1)
	expectedHeartbeat := Heartbeat{Job: "foo", Index: 0}

	go func() {
		errChan <- handler.SendToHealthManager("heartbeat", expectedHeartbeat)
	}()

	var err error
	select {
	case err = <-errChan:
	}
	assert.NoError(t, err)

	assert.Equal(t, len(client.PublishedMessages), 1)
	messages := client.PublishedMessages["hm.agent.heartbeat.my-agent-id"]

	assert.Equal(t, len(messages), 1)
	message := messages[0]

	expectedJson, _ := json.Marshal(expectedHeartbeat)
	assert.Equal(t, string(expectedJson), string(message.Payload))
}

func TestNatsHandlerConnectionInfo(t *testing.T) {
	settings := &fakesettings.FakeSettingsService{MbusUrl: "nats://foo:bar@127.0.0.1:1234"}
	client, handler := buildNatsClientAndHandler(settings)

	err := handler.Start(func(req boshhandler.Request) (res boshhandler.Response) { return })
	assert.NoError(t, err)
	defer handler.Stop()

	assert.NotNil(t, client.ConnectedConnectionProvider)

	connInfo := client.ConnectedConnectionProvider

	expectedConnInfo := &yagnats.ConnectionInfo{
		Addr:     "127.0.0.1:1234",
		Username: "foo",
		Password: "bar",
	}

	assert.Equal(t, connInfo, expectedConnInfo)
}

func TestNatsHandlerConnectionInfoDoesNotErrWhenNoUsernameAndPassword(t *testing.T) {
	settings := &fakesettings.FakeSettingsService{MbusUrl: "nats://127.0.0.1:1234"}
	_, handler := buildNatsClientAndHandler(settings)

	err := handler.Start(func(req boshhandler.Request) (res boshhandler.Response) { return })
	assert.NoError(t, err)
	defer handler.Stop()
}

func TestNatsHandlerConnectionInfoErrsWhenHasUsernameWithoutPassword(t *testing.T) {
	settings := &fakesettings.FakeSettingsService{MbusUrl: "nats://foo@127.0.0.1:1234"}
	_, handler := buildNatsClientAndHandler(settings)

	err := handler.Start(func(req boshhandler.Request) (res boshhandler.Response) { return })
	assert.Error(t, err)
	defer handler.Stop()
}

func buildNatsClientAndHandler(settings boshsettings.Service) (client *fakeyagnats.FakeYagnats, handler boshhandler.Handler) {
	logger := boshlog.NewLogger(boshlog.LEVEL_NONE)
	client = fakeyagnats.New()
	handler = NewNatsHandler(settings, logger, client)
	return
}
