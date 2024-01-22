run:
	# @templ generate
	cd app && docker compose up --build

deploy_infra:
	cd infra && pulumi up
