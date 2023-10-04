package commons

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"golang.org/x/xerrors"
)

type MDRepoTicket struct {
	IRODSTicket   string
	IRODSDataPath string
}

type MDRepoTicketObject struct {
	TicketString string `json:"ticket"`
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
		return MDRepoTicket{}, xerrors.Errorf("failed to parse ticket parts. must have two parts: %w", InvalidTicketError)
	}

	irodsTicket := ticketParts[0]
	irodsDataPath := ticketParts[1]

	if !isAsciiString(irodsTicket) {
		return MDRepoTicket{}, xerrors.Errorf("failed to parse iRODS ticket. iRODS ticket string %s is invalid: %w", irodsTicket, InvalidTicketError)
	}

	if !isPathString(irodsDataPath) {
		return MDRepoTicket{}, xerrors.Errorf("failed to parse iRODS data path. iRODS target path %s is invalid: %w", irodsDataPath, InvalidTicketError)
	}

	return MDRepoTicket{
		IRODSTicket:   irodsTicket,
		IRODSDataPath: irodsDataPath,
	}, nil
}

func GetMDRepoTicketsFromString(ticketString string) ([]MDRepoTicket, error) {
	tickets := strings.Split(ticketString, ";")
	if len(tickets) < 1 {
		return nil, xerrors.Errorf("failed to parse tickets: %w", InvalidTicketError)
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

func GetMDRepoTicketStringFromToken(token string) (string, error) {
	req, err := http.NewRequest("POST", mdRepoGetTicketApiUrl, nil)
	if err != nil {
		return "", xerrors.Errorf("failed to create a new request to retrieve tickets: %w", err)
	}

	req.Body = io.NopCloser(strings.NewReader(token))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", xerrors.Errorf("failed to perform http post to retrieve tickets: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return "", xerrors.Errorf("failed to retrieve tickets from token, not exist: %w", InvalidTokenError)
	}

	if resp.StatusCode >= 400 {
		return "", xerrors.Errorf("failed to retrieve tickets from token, http error %s", resp.Status)
	}

	ticketStringJsonBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", xerrors.Errorf("failed to retrieve tickets from token, read failed: %w", err)
	}

	ticketObject := MDRepoTicketObject{}
	err = json.Unmarshal(ticketStringJsonBytes, &ticketObject)
	if err != nil {
		return "", xerrors.Errorf("failed to unmarshal ticket object from JSON: %w", err)
	}

	return ticketObject.TicketString, nil
}
