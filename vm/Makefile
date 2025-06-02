build:
	nix build  ./#nixosConfigurations.vm.config.system.build.vm
run:
	./result/bin/run-nixos-vm
run-pf:
	QEMU_NET_OPTS="hostfwd=tcp::2222-:22" ./result/bin/run-nixos-vm
