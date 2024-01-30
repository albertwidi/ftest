#!/bin/sh

set -e

# Create 'test-fund'
curl \
	localhost:8080/v1/ledger/account \
	--request POST \
	--data @- << EOF
{
	"account_id": "test-fund",
	"account_type": "funding"
}
EOF

# Create 'test-acc-1'
curl \
	localhost:8080/v1/ledger/account \
	--request POST \
	--data @- << EOF
{
	"account_id": "test-acc-1"
}
EOF

# Create 'test-acc-1'
curl \
	localhost:8080/v1/ledger/account \
	--request POST \
	--data @- << EOF
{
	"account_id": "test-acc-2"
}
EOF

# Transfer to 'test-acc-1'
curl \
	localhost:8080/v1/ledger/transfer \
	--request POST \
	--data @- << EOF
{
	"from_account": "test-fund",
	"to_account": "test-acc-1",
	"amount": "10000"
}
EOF

# Transfer to 'test-acc-2'
curl \
	localhost:8080/v1/ledger/transfer \
	--request POST \
	--data @- << EOF
{
	"from_account": "test-fund",
	"to_account": "test-acc-1",
	"amount": "20000"
}
EOF