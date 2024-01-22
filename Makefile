.PHONY: app infra

app:
	# @templ generate
		cd app && docker compose up --build

infra:
		cd infra && pulumi up
