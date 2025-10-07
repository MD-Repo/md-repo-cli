package commons

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	log "github.com/sirupsen/logrus"
	"golang.org/x/xerrors"
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
	ticketParts := strings.Split(string(ticketString), ":")
	if len(ticketParts) != 2 {
		return MDRepoTicket{}, xerrors.Errorf("failed to parse ticket parts. must have two parts: %w", NewInvalidTicketError(ticketString))
	}

	irodsTicket := ticketParts[0]
	irodsDataPath := ticketParts[1]

	if !isAsciiString(irodsTicket) {
		return MDRepoTicket{}, xerrors.Errorf("failed to parse iRODS ticket. iRODS ticket string %q is invalid: %w", irodsTicket, NewInvalidTicketError(ticketString))
	}

	if !isPathString(irodsDataPath) {
		return MDRepoTicket{}, xerrors.Errorf("failed to parse iRODS data path. iRODS target path %q is invalid: %w", irodsDataPath, NewInvalidTicketError(ticketString))
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

	return "", xerrors.Errorf("failed to extract submission ID")
}

func GetMDRepoTicketsFromString(ticketString string) ([]MDRepoTicket, error) {
	tickets := strings.Split(ticketString, ";")
	if len(tickets) < 1 {
		return nil, xerrors.Errorf("failed to parse tickets: %w", NewInvalidTicketError(ticketString))
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

	if strings.HasPrefix(str, fmt.Sprintf("/%s/", mdRepoZone)) {
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
		"package":  "commons",
		"function": "GetMDRepoTicketStringFromToken",
	})

	apiURL := mdRepoURL + mdRepoGetTicketApi
	if len(serviceURL) > 0 {
		if !strings.HasPrefix(serviceURL, "http") {
			return "", xerrors.Errorf("failed to make API endpoint URL from non-http/s URL '%s'", serviceURL)
		}

		apiURL = strings.TrimRight(serviceURL, "/") + mdRepoGetTicketApi
	}

	logger.Debugf("Requesting to API server at '%s'", apiURL)

	req, err := http.NewRequest("POST", apiURL, nil)
	if err != nil {
		return "", xerrors.Errorf("failed to create a new request to retrieve tickets: %w", err)
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
			dialError := xerrors.Errorf("%s: %w", err.Error(), NewDialHTTPError(req.Host))
			return "", xerrors.Errorf("failed to perform http post to retrieve tickets: %w", dialError)
		}

		return "", xerrors.Errorf("failed to perform http post to retrieve tickets: %w", err)
	}

	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", xerrors.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != 200 {
		return "", NewMDRepoServiceError(string(responseBody))
	}

	// response body will be ticket object
	ticketObject := MDRepoTicketObject{}
	err = json.Unmarshal(responseBody, &ticketObject)
	if err != nil {
		return "", xerrors.Errorf("failed to unmarshal ticket object from JSON: %w", err)
	}

	return ticketObject.TicketString, nil
}
