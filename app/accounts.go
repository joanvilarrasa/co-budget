package app

import (
	"co-budget/data"
	"co-budget/lib"
)

func Accounts() string {
	accounts, accountsError := data.AccountGetAll(lib.SortConfig{Key: "Name", Direction: "asc"})
	errorMsg := ""
	if accountsError != data.AS_Ok {
		errorMsg = string(accountsError)
	}
	props := Props{
		"Error":        errorMsg,
		"AccountsList": accounts,
	}

	return lib.ParseHtmlTemplate("./app/accounts/page.html", props)
}
