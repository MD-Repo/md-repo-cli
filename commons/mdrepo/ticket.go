package mdrepo

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/MD-Repo/md-repo-cli/commons/config"
	"github.com/MD-Repo/md-repo-cli/commons/types"
	"github.com/cockroachdb/errors"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	log "github.com/sirupsen/logrus"
)

type MDRepoTicket struct {
	IRODSTicket   string
	IRODSDataPath string
}

type MDRepoTicketObject struct {
	TicketString string `json:"tickets"`
}

func GetMDRepoTicketString(tickets []MDRepoTicket) (string, error) {
	sb := strings.Builder{}
	for _, ticket := range tickets {
		if sb.Len() != 0 {
			sb.WriteString(";")
		}
		sb.WriteString(fmt.Sprintf("%s:%s", ticket.IRODSTicket, ticket.IRODSDataPath))
	}

	return sb.String(), nil
}

func GetMDRepoTicketFromString(ticketString string) (MDRepoTicket, error) {
	ticketParts := strings.Split(ticketString, ":")
	if len(ticketParts) != 2 {
		return MDRepoTicket{}, errors.Wrapf(types.NewInvalidTicketError(ticketString), "failed to parse ticket parts. must have two parts")
	}

	irodsTicket := ticketParts[0]
	irodsDataPath := ticketParts[1]

	if !isAsciiString(irodsTicket) {
		return MDRepoTicket{}, errors.Wrapf(types.NewInvalidTicketError(ticketString), "failed to parse iRODS ticket. iRODS ticket string %q is invalid", irodsTicket)
	}

	if !isPathString(irodsDataPath) {
		return MDRepoTicket{}, errors.Wrapf(types.NewInvalidTicketError(ticketString), "failed to parse iRODS data path. iRODS target path %q is invalid", irodsDataPath)
	}

	return MDRepoTicket{
		IRODSTicket:   irodsTicket,
		IRODSDataPath: irodsDataPath,
	}, nil
}

func GetMDRepoSimulationRelPath(irodsPath string) (string, error) {
	start := strings.LastIndex(irodsPath, "/MDR")
	if start >= 0 {
		return irodsPath[start+1:], nil
	}

	return "", errors.Errorf("failed to extract submission ID")
}

func GetMDRepoTicketsFromString(ticketString string) ([]MDRepoTicket, error) {
	tickets := strings.Split(ticketString, ";")
	if len(tickets) == 0 || len(ticketString) == 0 {
		return nil, errors.Wrapf(types.NewInvalidTicketError(ticketString), "failed to parse tickets")
	}

	mdRepoTickets := []MDRepoTicket{}
	for _, ticket := range tickets {
		mdRepoTicket, err := GetMDRepoTicketFromString(ticket)
		if err != nil {
			return nil, err
		}

		mdRepoTickets = append(mdRepoTickets, mdRepoTicket)
	}

	return mdRepoTickets, nil
}

func isPathString(str string) bool {
	if len(str) == 0 {
		return false
	}

	if strings.HasPrefix(str, fmt.Sprintf("/%s/", config.MDRepoZone)) {
		return true
	}
	return false
}

func isAsciiString(str string) bool {
	if len(str) == 0 {
		return false
	}

	for _, s := range str {
		sb := byte(s)
		if sb < '!' || sb > '~' {
			// non ascii
			return false
		}
	}
	return true
}

func GetMDRepoTicketStringFromToken(serviceURL string, token string) (string, error) {
	logger := log.WithFields(log.Fields{
		"service_url": serviceURL,
		"token":       token,
	})

	apiURL := config.MDRepoURL + config.MDRepoGetTicketApi
	if len(serviceURL) > 0 {
		if !strings.HasPrefix(serviceURL, "http") {
			return "", errors.Errorf("failed to make API endpoint URL from non-http/s URL %q", serviceURL)
		}

		apiURL = strings.TrimRight(serviceURL, "/") + config.MDRepoGetTicketApi
	}

	logger.Debugf("Requesting to API server at '%s'", apiURL)

	req, err := http.NewRequest("POST", apiURL, nil)
	if err != nil {
		return "", errors.Wrapf(err, "failed to create a new request to retrieve tickets")
	}

	req.Body = io.NopCloser(strings.NewReader(token))
	req.Header.Add("Accept", "*/*")
	req.Header.Add("Content-Type", "text/plain")
	req.ContentLength = int64(len(token))

	client := &http.Client{}
	transport := &http.Transport{
		Proxy:              http.ProxyFromEnvironment,
		DisableCompression: true,
	}
	client.Transport = transport

	resp, err := client.Do(req)
	if err != nil {
		if strings.Contains(err.Error(), "dial tcp") {
			dialError := errors.Join(err, types.NewDialHTTPError(req.Host))
			return "", errors.Wrapf(dialError, "failed to perform http post to retrieve tickets")
		}

		return "", errors.Wrapf(err, "failed to perform http post to retrieve tickets")
	}

	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Wrapf(err, "failed to read response body")
	}

	if resp.StatusCode != 200 {
		return "", types.NewMDRepoServiceError(string(responseBody))
	}

	// response body will be ticket object
	ticketObject := MDRepoTicketObject{}
	err = json.Unmarshal(responseBody, &ticketObject)
	if err != nil {
		return "", errors.Wrapf(err, "failed to unmarshal ticket object from JSON")
	}

	return ticketObject.TicketString, nil
}

func (ticket *MDRepoTicket) GetAccount() (*irodsclient_types.IRODSAccount, error) {
	ticketString := ""
	if ticket != nil {
		ticketString = ticket.IRODSTicket
	}

	return &irodsclient_types.IRODSAccount{
		AuthenticationScheme:    irodsclient_types.AuthSchemeNative,
		ClientServerNegotiation: false,
		CSNegotiationPolicy:     irodsclient_types.CSNegotiationPolicyRequestTCP,
		Host:                    config.MDRepoHost,
		Port:                    config.MDRepoPort,
		ClientUser:              config.MDRepoUser,
		ClientZone:              config.MDRepoZone,
		ProxyUser:               config.MDRepoUser,
		ProxyZone:               config.MDRepoZone,
		Password:                config.MDRepoUserPassword,
		Ticket:                  ticketString,
		DefaultResource:         "",
		DefaultHashScheme:       config.MDRepoHashScheme,
		PamTTL:                  1,
		SSLConfiguration:        nil,
	}, nil
}
