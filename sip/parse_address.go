package sip

import (
	"errors"
	"fmt"
	"strings"
)

// ParseAddressValue parses an address - such as from a From, To, or
// Contact header. It returns:
// See RFC 3261 section 20.10 for details on parsing an address.
// Note that this method will not accept a comma-separated list of addresses.
func ParseAddressValue(addressText string, uri *Uri, headerParams HeaderParams) (displayName string, err error) {
	// headerParams = NewParams()
	var semicolon, equal, startQuote, endQuote int = -1, -1, -1, -1
	var name string
	var uriStart, uriEnd int = 0, -1
	var inBrackets, inQuotesParamValue bool
	for i, c := range addressText {
		if inQuotesParamValue {
			if c == '"' {
				inQuotesParamValue = false
			}

			continue
		}

		switch c {
		case '"':
			if equal > 0 {
				inQuotesParamValue = true
				continue
			}

			if startQuote < 0 {
				startQuote = i
			} else {
				endQuote = i
			}
		case '<':
			if uriStart > 0 {
				// This must be additional options parsing
				continue
			}

			// display-name   =  *(token LWS)/ quoted-string
			if endQuote > 0 {
				displayName = addressText[startQuote+1 : endQuote]
				startQuote, endQuote = -1, -1
			} else {
				displayName = strings.TrimSpace(addressText[:i])
			}
			uriStart = i + 1
			inBrackets = true
		case '>':
			// uri can be without <> in that case there all after ; are header params
			uriEnd = i
			equal = -1
			inBrackets = false
		case ';':
			// uri can be without <> in that case there all after ; are header params
			if inBrackets {
				semicolon = i
				continue
			}

			if uriEnd < 0 {
				uriEnd = i
				semicolon = i
				continue
			}

			if equal > 0 {
				val := addressText[equal+1 : i]
				headerParams.Add(name, val)
			} else if semicolon > 0 {
				// Case when we have key name but not value. ex ;+siptag;
				name = addressText[semicolon+1 : i]
				headerParams.Add(name, "")
			}
			name = ""
			equal = 0
			semicolon = i

		case '=':
			name = addressText[semicolon+1 : i]
			equal = i
		case '*':
			if startQuote > 0 || uriStart > 0 {
				continue
			}
			uri = &Uri{
				Wildcard: true,
			}
			return
		}
	}

	if uriEnd < 0 {
		uriEnd = len(addressText)
	}

	if uriStart > uriEnd {
		return "", errors.New("Malormed URI")
	}

	err = ParseUri(addressText[uriStart:uriEnd], uri)
	if err != nil {
		return
	}

	if equal > 0 {
		val := addressText[equal+1:]
		headerParams.Add(name, val)
		name, val = "", ""
	}
	// params := strings.Split(addressText, ";")
	// if len(params) > 1 {
	// 	for _, section := range params[1:] {
	// 		arr := strings.Split(section, "=")
	// 		headerParams.Add(arr[0], arr[1])
	// 	}
	// }

	return
}

// parseToAddressHeader generates ToHeader
func parseToAddressHeader(headerName string, headerText string) (header Header, err error) {

	h := &ToHeader{
		Address: Uri{},
		Params:  NewParams(),
	}
	h.DisplayName, err = ParseAddressValue(headerText, &h.Address, h.Params)
	// h.DisplayName, h.Address, h.Params, err = ParseAddressValue(headerText)

	if h.Address.Wildcard {
		// The Wildcard '*' URI is only permitted in Contact headers.
		err = fmt.Errorf(
			"wildcard uri not permitted in to: header: %s",
			headerText,
		)
		return
	}
	return h, err
}

// parseFromAddressHeader generates FromHeader
func parseFromAddressHeader(headerName string, headerText string) (header Header, err error) {

	h := FromHeader{
		Address: Uri{},
		Params:  NewParams(),
	}
	h.DisplayName, err = ParseAddressValue(headerText, &h.Address, h.Params)
	// h.DisplayName, h.Address, h.Params, err = ParseAddressValue(headerText)
	if err != nil {
		return
	}

	if h.Address.Wildcard {
		// The Wildcard '*' URI is only permitted in Contact headers.
		err = fmt.Errorf(
			"wildcard uri not permitted in to: header: %s",
			headerText,
		)
		return
	}
	return &h, nil
}

// parseContactAddressHeader generates ContactHeader
func parseContactAddressHeader(headerName string, headerText string) (header Header, err error) {
	inBrackets := false
	inQuotes := false

	h := ContactHeader{
		Params: NewParams(),
	}

	endInd := len(headerText)
	end := endInd - 1

	for idx, char := range headerText {
		if char == '<' && !inQuotes {
			inBrackets = true
		} else if char == '>' && !inQuotes {
			inBrackets = false
		} else if char == '"' {
			inQuotes = !inQuotes
		} else if !inQuotes && !inBrackets {
			switch {
			case char == ',':
				err = errComaDetected(idx)
			case idx == end:
				endInd = idx + 1
			default:
				continue
			}

			break
		}
	}

	var e error
	h.DisplayName, e = ParseAddressValue(headerText[:endInd], &h.Address, h.Params)
	if e != nil {
		return nil, e
	}

	return &h, err
}

// parseRouteHeader generates RouteHeader
func parseRouteHeader(headerName string, headerText string) (header Header, err error) {
	// Append a comma to simplify the parsing code; we split address sections
	// on commas, so use a comma to signify the end of the final address section.
	h := RouteHeader{}
	parseRouteAddress(headerText, &h.Address)

	return &h, nil
}

// parseRouteHeader generates RecordRouteHeader
func parseRecordRouteHeader(headerName string, headerText string) (header Header, err error) {
	// Append a comma to simplify the parsing code; we split address sections
	// on commas, so use a comma to signify the end of the final address section.
	h := RecordRouteHeader{}
	parseRouteAddress(headerText, &h.Address)
	return &h, nil
}

func parseRouteAddress(headerText string, address *Uri) (err error) {
	inBrackets := false
	inQuotes := false
	end := len(headerText) - 1
	for idx, char := range headerText {
		if char == '<' && !inQuotes {
			inBrackets = true
			continue
		}
		if char == '>' && !inQuotes {
			inBrackets = false
		} else if char == '"' {
			inQuotes = !inQuotes
		}

		if !inQuotes && !inBrackets {
			switch {
			case char == ',':
				err = errComaDetected(idx)
			case idx == end:
				idx = idx + 1
			default:
				continue
			}

			_, e := ParseAddressValue(headerText[:idx], address, nil)
			if e != nil {
				return e
			}
			break
		}
	}
	return
}
