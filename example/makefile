.PHONY: dev dev/hurl dev/entr

dev:
	@make -sj dev/herl dev/entr

dev/herl:
	@herl -serve

dev/entr:
	@ls *.{go,html} | entr -rs 'herl -n; go run .'
