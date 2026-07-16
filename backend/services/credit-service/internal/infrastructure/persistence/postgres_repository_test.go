package persistence

import (
	"errors"
	"testing"

	domain "credit-service/internal/domain/credit"
)

func TestValidateTransferLedgerStateRejectsPartialTransfer(t *testing.T) {
	tests := []struct {
		name         string
		debitExists  bool
		creditExists bool
		wantErr      bool
	}{
		{name: "new transfer", debitExists: false, creditExists: false},
		{name: "idempotent transfer", debitExists: true, creditExists: true},
		{name: "debit only", debitExists: true, creditExists: false, wantErr: true},
		{name: "credit only", debitExists: false, creditExists: true, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTransferLedgerState(tt.debitExists, tt.creditExists)
			if tt.wantErr {
				if !errors.Is(err, domain.ErrInconsistentCreditTransfer) {
					t.Fatalf("validateTransferLedgerState() error = %v, want inconsistent transfer", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("validateTransferLedgerState() error = %v, want nil", err)
			}
		})
	}
}

func TestValidateTransferBalanceRejectsUnbalancedTransfer(t *testing.T) {
	tests := []struct {
		name    string
		debit   int64
		credit  int64
		wantErr bool
	}{
		{name: "balanced", debit: -50, credit: 50},
		{name: "over credit", debit: -50, credit: 80, wantErr: true},
		{name: "under credit", debit: -50, credit: 30, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTransferBalance(
				domain.LedgerEntry{Delta: tt.debit},
				domain.LedgerEntry{Delta: tt.credit},
			)
			if tt.wantErr {
				if !errors.Is(err, domain.ErrUnbalancedCreditTransfer) {
					t.Fatalf("validateTransferBalance() error = %v, want unbalanced transfer", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("validateTransferBalance() error = %v, want nil", err)
			}
		})
	}
}
