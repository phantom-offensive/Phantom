package implant

// ══════════════════════════════════════════
//  BOF CATALOG — Built-in BOF Library
// ══════════════════════════════════════════
// Reference catalog of available BOFs organized by category.
// BOFs are loaded from disk or embedded at compile time.
// Compatible with Cobalt Strike BOF format (.o COFF files).

// BOFEntry describes a single BOF in the catalog.
type BOFEntry struct {
	Name        string   `json:"name"`
	Category    string   `json:"category"`
	Description string   `json:"description"`
	Args        string   `json:"args"`
	Author      string   `json:"author"`
	MITRE       string   `json:"mitre"`
}

// BOFCatalog returns all available BOFs organized by category.
func BOFCatalog() []BOFEntry {
	return []BOFEntry{
		// ──── Active Directory Enumeration ────
		{Name: "ad-users", Category: "AD Enum", Description: "Enumerate all domain users via LDAP", MITRE: "T1087.002"},
		{Name: "ad-computers", Category: "AD Enum", Description: "Enumerate all domain computers", MITRE: "T1018"},
		{Name: "ad-groups", Category: "AD Enum", Description: "Enumerate all domain groups", MITRE: "T1069.002"},
		{Name: "ad-spns", Category: "AD Enum", Description: "Enumerate Service Principal Names", MITRE: "T1558.003"},
		{Name: "ad-gpos", Category: "AD Enum", Description: "Enumerate Group Policy Objects", MITRE: "T1615"},
		{Name: "ad-trusts", Category: "AD Enum", Description: "Enumerate domain trusts", MITRE: "T1482"},
		{Name: "ad-ous", Category: "AD Enum", Description: "Enumerate Organizational Units", MITRE: "T1087.002"},
		{Name: "ad-delegations", Category: "AD Enum", Description: "Find delegation configurations", MITRE: "T1134"},
		{Name: "ad-dns", Category: "AD Enum", Description: "Enumerate DNS records via LDAP", MITRE: "T1018"},
		{Name: "ad-laps", Category: "AD Enum", Description: "Read LAPS passwords from AD", MITRE: "T1552.006"},

		// ──── AD ACL Enumeration ────
		{Name: "enum-acls", Category: "AD ACL", Description: "Enumerate object ACLs for privilege escalation paths", MITRE: "T1069"},
		{Name: "enum-dcsync", Category: "AD ACL", Description: "Find accounts with DCSync (replication) rights", MITRE: "T1003.006"},
		{Name: "enum-gmsa", Category: "AD ACL", Description: "Enumerate Group Managed Service Accounts", MITRE: "T1552"},
		{Name: "enum-rbcd", Category: "AD ACL", Description: "Find Resource-Based Constrained Delegation", MITRE: "T1134"},

		// ──── ADCS ────
		{Name: "adcs-enum", Category: "ADCS", Description: "Enumerate Certificate Authority and templates", MITRE: "T1649"},
		{Name: "adcs-vuln", Category: "ADCS", Description: "Check for vulnerable certificate templates (ESC1-ESC8)", MITRE: "T1649"},
		{Name: "adcs-request", Category: "ADCS", Description: "Request certificate from CA", Args: "/template:User /altname:admin", MITRE: "T1649"},

		// ──── Credential Dumping ────
		{Name: "nanodump", Category: "Credentials", Description: "MiniDump of lsass.exe (in-memory)", MITRE: "T1003.001"},
		{Name: "hashdump", Category: "Credentials", Description: "Dump SAM database hashes", MITRE: "T1003.002"},
		{Name: "credman", Category: "Credentials", Description: "Dump Windows Credential Manager", MITRE: "T1555.004"},
		{Name: "dumpntlm", Category: "Credentials", Description: "Extract NTLM hashes from memory", MITRE: "T1003"},
		{Name: "hivesave", Category: "Credentials", Description: "Save SAM/SYSTEM/SECURITY registry hives", MITRE: "T1003.002"},
		{Name: "autologon", Category: "Credentials", Description: "Read AutoLogon credentials from registry", MITRE: "T1552.002"},
		{Name: "wifidump", Category: "Credentials", Description: "Dump saved WiFi passwords", MITRE: "T1555"},
		{Name: "dumpclip", Category: "Credentials", Description: "Dump current clipboard contents", MITRE: "T1115"},

		// ──── Kerberos ────
		{Name: "asktgt", Category: "Kerberos", Description: "Request a TGT for a user", Args: "/user:admin /pass:password /domain:corp.local", MITRE: "T1558"},
		{Name: "asreproast", Category: "Kerberos", Description: "AS-REP roast accounts without preauth", MITRE: "T1558.004"},
		{Name: "kerberoast", Category: "Kerberos", Description: "Request service tickets for offline cracking", MITRE: "T1558.003"},
		{Name: "klist", Category: "Kerberos", Description: "List cached Kerberos tickets", MITRE: "T1558"},

		// ──── Evasion ────
		{Name: "sysmon-config", Category: "Evasion", Description: "Dump Sysmon configuration", MITRE: "T1562"},
		{Name: "kill-sysmon", Category: "Evasion", Description: "Unload Sysmon driver", MITRE: "T1562.001"},
		{Name: "check-debugger", Category: "Evasion", Description: "Check if being debugged", MITRE: "T1622"},
		{Name: "get-exclusions", Category: "Evasion", Description: "Get Windows Defender exclusion paths", MITRE: "T1562.001"},
		{Name: "get-av", Category: "Evasion", Description: "Enumerate installed security products", MITRE: "T1518.001"},

		// ──── Privilege Escalation ────
		{Name: "privesc-check", Category: "PrivEsc", Description: "Check AlwaysInstallElevated, unquoted paths, writable services", MITRE: "T1574"},
		{Name: "token-privs", Category: "PrivEsc", Description: "List current token privileges", MITRE: "T1134"},
		{Name: "hijackable-paths", Category: "PrivEsc", Description: "Find DLL hijackable paths", MITRE: "T1574.001"},
		{Name: "modifiable-services", Category: "PrivEsc", Description: "Find services writable by current user", MITRE: "T1574.011"},

		// ──── Networking ────
		{Name: "arp", Category: "Network", Description: "Display ARP table", MITRE: "T1016"},
		{Name: "ipconfig", Category: "Network", Description: "Network interface configuration", MITRE: "T1016"},
		{Name: "netstat", Category: "Network", Description: "Active network connections", MITRE: "T1049"},
		{Name: "netshares", Category: "Network", Description: "Enumerate SMB shares on target", Args: "\\\\target", MITRE: "T1135"},
		{Name: "portscan", Category: "Network", Description: "TCP port scan", Args: "target ports", MITRE: "T1046"},
		{Name: "routeprint", Category: "Network", Description: "Print routing table", MITRE: "T1016"},
		{Name: "listdns", Category: "Network", Description: "List DNS cache entries", MITRE: "T1016.001"},
		{Name: "firewall-rules", Category: "Network", Description: "List Windows Firewall rules", MITRE: "T1016"},

		// ──── Situational Awareness ────
		{Name: "whoami", Category: "Recon", Description: "Current user and privileges", MITRE: "T1033"},
		{Name: "env", Category: "Recon", Description: "Environment variables", MITRE: "T1082"},
		{Name: "driversigs", Category: "Recon", Description: "Loaded drivers with signatures (detect EDR)", MITRE: "T1518.001"},
		{Name: "enumdotnet", Category: "Recon", Description: "Enumerate .NET versions installed", MITRE: "T1518"},
		{Name: "enumdrives", Category: "Recon", Description: "Enumerate logical drives", MITRE: "T1082"},
		{Name: "sessions", Category: "Recon", Description: "Enumerate local sessions", MITRE: "T1033"},
		{Name: "listmods", Category: "Recon", Description: "List loaded modules in current process", MITRE: "T1057"},
		{Name: "dir", Category: "Recon", Description: "Directory listing with ACLs", Args: "C:\\path", MITRE: "T1083"},
		{Name: "recentfiles", Category: "Recon", Description: "List recently accessed files", MITRE: "T1083"},
	}
}

// GetBOFsByCategory returns BOFs filtered by category.
func GetBOFsByCategory(category string) []BOFEntry {
	var result []BOFEntry
	for _, b := range BOFCatalog() {
		if b.Category == category {
			result = append(result, b)
		}
	}
	return result
}

// GetBOFCategories returns unique BOF category names.
func GetBOFCategories() []string {
	seen := map[string]bool{}
	var cats []string
	for _, b := range BOFCatalog() {
		if !seen[b.Category] {
			cats = append(cats, b.Category)
			seen[b.Category] = true
		}
	}
	return cats
}
