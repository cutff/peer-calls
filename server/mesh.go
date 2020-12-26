package server

import (
	"net/http"

	"github.com/juju/errors"
)

type ReadyMessage struct {
	UserID string `json:"userId"`
	Room   string `json:"room"`
}

func NewMeshHandler(loggerFactory LoggerFactory, wss *WSS) http.Handler {
	log := loggerFactory.GetLogger("mesh")

	fn := func(w http.ResponseWriter, r *http.Request) {
		sub, err := wss.Subscribe(w, r)
		if err != nil {
			log.Printf("Error subscribing to websocket messages: %s", err)
		}

		for msg := range sub.Messages {
			adapter := sub.Adapter
			room := sub.Room
			clientID := sub.ClientID

			var (
				responseEventName MessageType
				err               error
			)

			switch msg.Type {
			case MessageTypeHangUp:
				log.Printf("[%s] hangUp event", clientID)
				adapter.SetMetadata(clientID, "")
			case MessageTypeReady:
				// FIXME check for errors
				payload, _ := msg.Payload.(map[string]interface{})
				adapter.SetMetadata(clientID, payload["nickname"].(string))

				clients, readyClientsErr := getReadyClients(adapter)
				if readyClientsErr != nil {
					log.Printf("Error retrieving clients: %s", readyClientsErr)
				}

				log.Printf("Got clients: %s", clients)

				err = adapter.Broadcast(
					NewMessage(MessageTypeUsers, room, map[string]interface{}{
						"initiator": clientID,
						"peerIds":   clientsToPeerIDs(clients),
						"nicknames": clients,
					}),
				)
				err = errors.Annotatef(err, "ready broadcast")
			case MessageTypeSignal:
				payload, _ := msg.Payload.(map[string]interface{})
				signal := payload["signal"]
				targetClientID, _ := payload["userId"].(string)

				log.Printf("Send signal from: %s to %s", clientID, targetClientID)
				err = adapter.Emit(targetClientID, NewMessage(MessageTypeSignal, room, map[string]interface{}{
					"userId": clientID,
					"signal": signal,
				}))
				err = errors.Annotatef(err, "signal emit")
			}

			if err != nil {
				log.Printf("Error sending event (event: %s, room: %s, source: %s)", responseEventName, room, clientID)
			}
		}
	}
	return http.HandlerFunc(fn)
}

func getReadyClients(adapter Adapter) (map[string]string, error) {
	filteredClients := map[string]string{}
	clients, err := adapter.Clients()
	if err != nil {
		return filteredClients, errors.Annotate(err, "ready clients")
	}

	for clientID, nickname := range clients {
		// if nickame hasn't been set, the peer hasn't emitted ready yet so we
		// don't connect to that peer.
		if nickname != "" {
			filteredClients[clientID] = nickname
		}
	}
	return filteredClients, nil
}

func clientsToPeerIDs(clients map[string]string) (peers []string) {
	for clientID := range clients {
		peers = append(peers, clientID)
	}
	return
}
