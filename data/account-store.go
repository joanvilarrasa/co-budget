package data

import (
	"co-budget/lib"
	"context"
	"database/sql"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"
)

type AccountType string

const (
	LTB AccountType = "LTB"
	MTB AccountType = "MTB"
	STB AccountType = "STB"
)

func parseAccountType(s string) (AccountType, bool) {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case string(LTB):
		return LTB, true
	case string(MTB):
		return MTB, true
	case string(STB):
		return STB, true
	default:
		return "", false
	}
}

type Account struct {
	ID             int64
	Name           string
	Description    string
	CreatedAt      string
	InitialBalance float64
	Type           AccountType
	CurrentBalance float64
}

type AccountStoreResponse string

const (
	AS_AccountStoreNil    AccountStoreResponse = "AS_AccountStoreNil"
	AS_AccountStoreOff    AccountStoreResponse = "AS_AccountStoreOff"
	AS_DBError            AccountStoreResponse = "AS_DBError"
	AS_AccountDisappeared AccountStoreResponse = "AS_AccountDisappeared"
	AS_AccountNotFound    AccountStoreResponse = "AS_AccountNotFound"
	AS_CreateBadInput     AccountStoreResponse = "AS_CreateBadInput"
	AS_Ok                 AccountStoreResponse = "AS_Ok"
)

type AccountStore struct {
	mu       sync.Mutex
	initErr  error
	db       *sql.DB
	ctx      context.Context
	accounts map[int64]Account
}

var store *AccountStore

func InitAccountStore(db *sql.DB, ctx context.Context, initErr error) {
	log.Printf("[account-store] InitAccountStore init")
	if db == nil {
		initErr = fmt.Errorf("[account-store] db is nil")
	}
	if ctx == nil {
		initErr = fmt.Errorf("[account-store] db context is nil")
	}
	store = &AccountStore{
		initErr:  initErr,
		db:       db,
		ctx:      ctx,
		accounts: map[int64]Account{},
	}
	if store.initErr != nil {
		log.Printf("[account-store] init error: %v", store.initErr)
	}
	if dbAccountsErr := queryAllAccountsFromDB(); dbAccountsErr != AS_Ok {
		log.Printf("[account-store] db failed to retrieve all the accounts")
		store.initErr = fmt.Errorf("[account-store] db failed to retrieve all the accounts")
	}
	log.Printf("[account-store] InitAccountStore end ok")
}

func (s *AccountStore) isAccountStoreActive() AccountStoreResponse {
	if s == nil {
		return AS_AccountStoreNil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.initErr != nil {
		return AS_AccountStoreOff
	}
	return AS_Ok
}

func queryAllAccountsFromDB() AccountStoreResponse {
	log.Printf("[account-store] queryAllAccountsFromDB init")
	if isActive := store.isAccountStoreActive(); isActive != AS_Ok {
		log.Printf("[account-store] failed to get accounts because %s", isActive)
		return isActive
	}
	accountRows, acccountRowsErr := store.db.QueryContext(
		store.ctx,
		`SELECT id, name, description, strftime('%Y-%m-%d %H:%M:%S', created_at), initial_balance, type FROM accounts ORDER BY created_at DESC, id DESC`,
	)
	if acccountRowsErr != nil {
		log.Printf("[account-store] failed to get accounts because %s", acccountRowsErr)
		return AS_DBError
	}
	defer accountRows.Close()

	store.mu.Lock()
	for accountRows.Next() {
		account := Account{
			CurrentBalance: 0,
		}
		if scanErr := accountRows.Scan(
			&account.ID,
			&account.Name,
			&account.Description,
			&account.CreatedAt,
			&account.InitialBalance,
			&account.Type,
		); scanErr != nil {
			log.Printf("[account-store] failed to get accounts because %s", scanErr)
			return AS_DBError
		}
		store.accounts[account.ID] = account
	}
	defer store.mu.Unlock()
	if rowsErr := accountRows.Err(); rowsErr != nil {
		log.Printf("[account-store] failed to get accounts because %s", rowsErr)
		return AS_DBError
	}
	log.Printf("[account-store] queryAllAccountsFromDB end ok")
	return AS_Ok
}

func AccountCreate(acName string, acDesc string, acInitBalance float64, acType string) (AccountStoreResponse, string) {
	log.Printf("[account-store] AccountCreate init")
	if isActive := store.isAccountStoreActive(); isActive != AS_Ok {
		log.Printf("[account-store] failed to create account because %s", isActive)
		return isActive, fmt.Sprintf("failed to create account because %s", isActive)
	}

	if acName == "" {
		return AS_CreateBadInput, "Name cannot be empty"
	}

	name := strings.TrimSpace(acName)
	description := strings.TrimSpace(acDesc)
	initialBalance := acInitBalance
	accountType, wasCorrectAccountType := parseAccountType(acType)

	if !wasCorrectAccountType {
		log.Printf("[account-store] The account type was not correct %s", acType)
	}

	result, err := store.db.ExecContext(
		store.ctx,
		`INSERT INTO accounts(name, description, initial_balance, type) VALUES(?, ?, ?, ?)`,
		name,
		description,
		initialBalance,
		accountType,
	)
	if err != nil {
		log.Printf("[account-store] failed to create account because %s", err)
		return AS_DBError, fmt.Sprintf("failed to create account because %s", err)
	}
	id, idErr := result.LastInsertId()
	if idErr != nil {
		log.Printf("[account-store] failed to create account because %s", err)
		return AS_DBError, fmt.Sprintf("failed to create account: %s", idErr)
	}
	account := Account{
		ID:             id,
		Name:           name,
		Description:    description,
		CreatedAt:      time.Now().Format(time.RFC3339),
		InitialBalance: initialBalance,
		Type:           accountType,
		CurrentBalance: 0,
	}
	store.mu.Lock()
	store.accounts[id] = account
	store.mu.Unlock()

	log.Printf("[account-store] AccountCreate end ok")
	return AS_Ok, ""
}

func AccountUpdate(id int64, acName string, acDescription string, initialBalance float64, acType string) (AccountStoreResponse, string) {
	log.Printf("[account-store] AccountUpdate init")
	if isActive := store.isAccountStoreActive(); isActive != AS_Ok {
		log.Printf("[account-store] failed to update account because %s", isActive)
		return isActive, fmt.Sprintf("failed to create account because %s", isActive)
	}

	// Load existing account to fill in missing/optional fields
	store.mu.Lock()
	account, ok := store.accounts[id]
	store.mu.Unlock()
	if !ok {
		log.Printf("[account-store] failed to update account because account not found")
		return AS_AccountNotFound, "Account not found"
	}

	updatedName := strings.TrimSpace(acName)
	updatedDescription := strings.TrimSpace(acDescription)
	updatedInitialBalance := initialBalance
	updatedType, wasCorrectAccountType := parseAccountType(acType)
	if !wasCorrectAccountType {
		log.Printf("[account-store] The account type was not correct %s", acType)
	}

	_, err := store.db.ExecContext(
		store.ctx,
		`UPDATE accounts SET name = ?, description = ?, initial_balance = ?, type = ? WHERE id = ?`,
		updatedName,
		updatedDescription,
		updatedInitialBalance,
		updatedType,
		id,
	)
	if err != nil {
		log.Printf("[account-store] failed to update account because %s", err)
		return AS_DBError, fmt.Sprintf("failed to update account: %s", err)
	}

	// Update in-memory account
	store.mu.Lock()
	defer store.mu.Unlock()
	account, ok = store.accounts[id]
	if ok {
		account.Name = updatedName
		account.Description = updatedDescription
		account.InitialBalance = updatedInitialBalance
		account.Type = updatedType
		store.accounts[id] = account
	} else {
		log.Printf("[account-store] failed to update account because disappeared (after db update)")
		return AS_AccountDisappeared, "Account disappeared after DB update"
	}

	log.Printf("[account-store] AccountUpdate end ok")
	return AS_Ok, ""
}

func AccountDelete(id int64) AccountStoreResponse {
	log.Printf("[account-store] AccountDelete init")
	if isActive := store.isAccountStoreActive(); isActive != AS_Ok {
		log.Printf("[account-store] failed to delete account because %s", isActive)
		return isActive
	}
	_, err := store.db.ExecContext(
		store.ctx,
		`DELETE FROM accounts WHERE id = ?`,
		id,
	)
	if err != nil {
		log.Printf("[account-store] failed to delete account because %s", err)
		return AS_DBError
	}
	store.mu.Lock()
	delete(store.accounts, id)
	store.mu.Unlock()

	log.Printf("[account-store] AccountDelete end ok")
	return AS_Ok
}

func AccountGetAll(sortConfig ...lib.SortConfig) ([]Account, AccountStoreResponse) {
	accounts := make([]Account, 0, len(store.accounts))
	log.Printf("[account-store] AccountGetAll init")
	if isActive := store.isAccountStoreActive(); isActive != AS_Ok {
		log.Printf("[account-store] failed to get all accounts because %s", isActive)
		return accounts, isActive
	}
	for _, acc := range store.accounts {
		accounts = append(accounts, acc)
	}

	// Apply sorting if a SortConfig is provided
	if len(sortConfig) > 0 {
		sc := sortConfig[0]
		sort.Slice(accounts, func(i, j int) bool {
			switch sc.Key {
			case "Name":
				if sc.Direction == lib.SortDesc {
					return accounts[i].Name > accounts[j].Name
				}
				return accounts[i].Name < accounts[j].Name
			case "CreatedAt":
				if sc.Direction == lib.SortDesc {
					return accounts[i].CreatedAt > accounts[j].CreatedAt
				}
				return accounts[i].CreatedAt < accounts[j].CreatedAt
			case "InitialBalance":
				if sc.Direction == lib.SortDesc {
					return accounts[i].InitialBalance > accounts[j].InitialBalance
				}
				return accounts[i].InitialBalance < accounts[j].InitialBalance
			case "Type":
				if sc.Direction == lib.SortDesc {
					return accounts[i].Type > accounts[j].Type
				}
				return accounts[i].Type < accounts[j].Type
			case "CurrentBalance":
				if sc.Direction == lib.SortDesc {
					return accounts[i].CurrentBalance > accounts[j].CurrentBalance
				}
				return accounts[i].CurrentBalance < accounts[j].CurrentBalance
			default:
				return accounts[i].ID < accounts[j].ID // default sort by ID ascending
			}
		})
	}

	log.Printf("[account-store] AccountGetAll end ok")
	return accounts, AS_Ok
}

func AccountGetOne(id int64) (*Account, AccountStoreResponse) {
	log.Printf("[account-store] AccountGetOne init")
	if isActive := store.isAccountStoreActive(); isActive != AS_Ok {
		log.Printf("[account-store] failed to get one account because %s", isActive)
		return nil, isActive
	}
	account, ok := store.accounts[id]
	if ok {
		log.Printf("[account-store] AccountGetOne end ok")
		return &account, AS_Ok
	}
	log.Printf("[account-store] failed to get one account because account not found")
	return nil, AS_AccountNotFound
}
