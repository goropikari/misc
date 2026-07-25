.PHONY: submodule-update

submodule-update:
	git submodule update --init --remote --merge --recursive
