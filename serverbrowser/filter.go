package serverbrowser

import (
	"owfc/filter"
	"owfc/logging"

	"github.com/logrusorgru/aurora/v3"
)

// DWC makes requests in the following formats:
// Matching ver 03: dwc_mver = %d and dwc_pid != %u and maxplayers = %d and numplayers < %d and dwc_mtype = %d and dwc_hoststate = %u and dwc_suspend = %u and (%s)
// Matching ver 90: dwc_mver = %d and dwc_pid != %u and maxplayers = %d and numplayers < %d and dwc_mtype = %d and dwc_mresv != dwc_pid and (%s)
// ...OR
// Self Lookup: dwc_pid = %u

// Example: dwc_mver = 90 and dwc_pid != 43 and maxplayers = 11 and numplayers < 11 and dwc_mtype = 0 and dwc_hoststate = 2 and dwc_suspend = 0 and (rk = 'vs' and ev >= 4250 and ev <= 5750 and p = 0)

func filterServers(moduleName string, servers []map[string]string, queryGame string, expression string) []map[string]string {
	// Matchmaking search
	tree, err := filter.Parse(expression)
	if err != nil {
		logging.Error(moduleName, "Error parsing filter:", err.Error())
		return []map[string]string{}
	}

	var filtered []map[string]string

	for _, server := range servers {
		if server["gamename"] != queryGame {
			continue
		}

		ret, err := filter.Eval(tree, server, queryGame)
		if err != nil {
			logging.Error(moduleName, "Error evaluating filter:", err.Error())
			return []map[string]string{}
		}

		if ret != 0 {
			filtered = append(filtered, server)
		}
	}

	if len(filtered) != 0 {
		logging.Info(moduleName, "Matched", aurora.BrightCyan(len(filtered)), "servers")
	}

	return filtered
}
