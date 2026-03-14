package app

import "co-budget/lib"

type Props map[string]interface{}

type HomeProps struct {
	AccountsPage string
}

func Layout() string {
	layoutdata := Props{
		"AccountsPage": Accounts(),
	}
	return lib.ParseHtmlTemplate("./app/layout.html", layoutdata)
}
