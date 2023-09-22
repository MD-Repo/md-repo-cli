package commons

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"

	log "github.com/sirupsen/logrus"
	"golang.org/x/xerrors"
)

var (
	WrongPasswordError error = xerrors.Errorf("wrong password")
	InvalidTicketError error = xerrors.Errorf("invalid ticket string")
)

type MDRepoTicket struct {
	IRODSTicket   string
	IRODSDataPath string
}

func EncodeMDRepoTickets(tickets []MDRepoTicket, password string) (string, error) {
	sb := strings.Builder{}
	for _, ticket := range tickets {
		if sb.Len() != 0 {
			sb.WriteString(";")
		}
		sb.WriteString(fmt.Sprintf("%s:%s", ticket.IRODSTicket, ticket.IRODSDataPath))
	}

	payload := sb.String()
	rawTicket, err := AesEncrypt(password, []byte(payload))
	if err != nil {
		return "", xerrors.Errorf("failed to AES encode ticket string: %w", err)
	}

	ticketString := base64.StdEncoding.EncodeToString(rawTicket)
	return ticketString, nil
}

func ValidateMDRepoTicket(ticketString string) error {
	logger := log.WithFields(log.Fields{
		"package":  "commons",
		"function": "ValidateMDRepoTicket",
	})

	if !isAsciiString(ticketString) {
		return xerrors.Errorf("failed to decode ticket string, contains non-ascii data: %w", InvalidTicketError)
	}

	logger.Debugf("decoded ticket data (in string): '%s'", ticketString)

	tickets := strings.Split(ticketString, ";")
	if len(tickets) < 1 {
		return xerrors.Errorf("failed to parse tickets: %w", InvalidTicketError)
	}

	for _, ticket := range tickets {
		ticketParts := strings.Split(string(ticket), ":")
		if len(ticketParts) != 2 {
			return xerrors.Errorf("failed to parse ticket parts. must have two parts: %w", InvalidTicketError)
		}

		irodsTicket := ticketParts[0]
		irodsDataPath := ticketParts[1]

		logger.Debugf("extracted iRODS Ticket %s, iRODS Path %s", irodsTicket, irodsDataPath)

		if !isPathString(irodsDataPath) {
			return xerrors.Errorf("failed to parse iRODS data path. iRODS target path %s is invalid: %w", irodsDataPath, InvalidTicketError)
		}
	}

	return nil
}

func GetMDRepoTicketFromPlainText(ticket string) (MDRepoTicket, error) {
	ticketParts := strings.Split(string(ticket), ":")
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

func GetMDRepoTicketsFromPlainText(ticketString string) ([]MDRepoTicket, error) {
	tickets := strings.Split(ticketString, ";")
	if len(tickets) < 1 {
		return nil, xerrors.Errorf("failed to parse tickets: %w", InvalidTicketError)
	}

	mdRepoTickets := []MDRepoTicket{}
	for _, ticket := range tickets {
		mdRepoTicket, err := GetMDRepoTicketFromPlainText(ticket)
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

func DecodeMDRepoTickets(tickets string, password string) ([]MDRepoTicket, error) {
	logger := log.WithFields(log.Fields{
		"package":  "commons",
		"function": "DecodeMDRepoTickets",
	})

	rawTicket, err := base64.StdEncoding.DecodeString(tickets)
	if err != nil {
		return nil, xerrors.Errorf("failed to Base64 decode ticket string '%s': %w", tickets, err)
	}

	logger.Debugf("raw encrypted ticket data (in hex): '%x'\n", rawTicket)

	payload, err := AesDecrypt(password, rawTicket)
	if err != nil {
		return nil, xerrors.Errorf("failed to AES decode ticket string: %w", err)
	}

	logger.Debugf("decoded ticket data (in hex): '%x'", payload)

	err = ValidateMDRepoTicket(string(payload))
	if err != nil {
		logger.Error(err)
		return nil, xerrors.Errorf("failed to validate ticket data (in hex) '%x': %w", payload, WrongPasswordError)
	}

	return GetMDRepoTicketsFromPlainText(string(payload))
}

func ReadTicketsFromDownloadHash(config *Config, downloadHash string) ([]MDRepoTicket, bool, error) {
	mdRepoTickets := []MDRepoTicket{}

	// check if this is download hash
	req, err := http.NewRequest("GET", mdRepoDownloadHashApiUrl+"/"+downloadHash, nil)
	if err != nil {
		return nil, false, xerrors.Errorf("failed to create a new request to retrieve download tickets: %w", err)
	}

	req.Header.Add("Accept", "text/plain")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, false, xerrors.Errorf("failed to perform http get to retrieve download tickets: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, true, xerrors.Errorf("failed to retrieve download tickets from hash, not exist")
	}

	if resp.StatusCode >= 400 {
		return nil, false, xerrors.Errorf("failed to retrieve download tickets from hash, http error %s", resp.Status)
	}

	ticketStringBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false, xerrors.Errorf("failed to retrieve download tickets from hash, read failed: %w", err)
	}

	ticketString := string(ticketStringBytes)
	if len(strings.TrimSpace(ticketString)) == 0 {
		return nil, false, xerrors.Errorf("failed to retrieve download tickets from hash, content empty: %w", err)
	}

	tickets, err := config.GetMDRepoTickets(ticketString)
	if err != nil {
		return nil, false, xerrors.Errorf("failed to parse MD-Repo Ticket: %w", err)
	}

	mdRepoTickets = append(mdRepoTickets, tickets...)

	if len(mdRepoTickets) == 0 {
		return nil, false, xerrors.Errorf("failed to parse MD-Repo Ticket. No ticket is provided")
	}

	return mdRepoTickets, false, nil
}

func ReadTicketsFromStringOrDownloadHash(config *Config, ticketString string) ([]MDRepoTicket, error) {
	// check if this is download hash
	mdRepoTickets, checkString, err := ReadTicketsFromDownloadHash(config, ticketString)
	if !checkString {
		if err != nil {
			return nil, err
		}

		// got tickets
		return mdRepoTickets, nil
	}

	// given string is ticket string
	mdRepoTickets = []MDRepoTicket{}

	tickets, err := config.GetMDRepoTickets(ticketString)
	if err != nil {
		return nil, xerrors.Errorf("failed to parse MD-Repo Ticket: %w", err)
	}

	mdRepoTickets = append(mdRepoTickets, tickets...)

	if len(mdRepoTickets) == 0 {
		return nil, xerrors.Errorf("failed to parse MD-Repo Ticket. No ticket is provided")
	}

	return mdRepoTickets, nil
}

func ReadTicketsFromString(config *Config, ticketString string) ([]MDRepoTicket, error) {
	mdRepoTickets := []MDRepoTicket{}

	tickets, err := config.GetMDRepoTickets(ticketString)
	if err != nil {
		return nil, xerrors.Errorf("failed to parse MD-Repo Ticket: %w", err)
	}

	mdRepoTickets = append(mdRepoTickets, tickets...)

	if len(mdRepoTickets) == 0 {
		return nil, xerrors.Errorf("failed to parse MD-Repo Ticket. No ticket is provided")
	}

	return mdRepoTickets, nil
}
