package app

import (
	"co-budget/data"
	"co-budget/lib"
)

func Accounts() string {
	props := Props{
		"Error": "",
		"Table": AccountsTable(),
	}

	return lib.ParseHtmlTemplate("./app/accounts/page.html", props)
}

func AccountsTable() string {
	accounts, _ := data.AccountGetAll()

	props := Props{
		"AccountsList": accounts,
	}

	return lib.ParseHtmlTemplate("./app/accounts/table.html", props)
}
