# Build this VM with nix build  ./#nixosConfigurations.vm.config.system.build.vm
# Then run is with: ./result/bin/run-nixos-vm
# To be able to connect with ssh enable port forwarding with:
# QEMU_NET_OPTS="hostfwd=tcp::2222-:22" ./result/bin/run-nixos-vm
# Then connect with ssh -p 2222 guest@localhost
{
  lib,
  config,
  pkgs,
  ...
}:
let
  # Import your Go application package definition
  # This makes 'my-go-app' available as a Nix package.
  myGoApp = import ./my-go-app { inherit pkgs; };
in
{
  # Internationalisation options
  i18n.defaultLocale = "en_US.UTF-8";
  console.keyMap = "fr";

  # Options for the screen
  virtualisation.vmVariant = {
    virtualisation.qemu.options = [
      "-nographic" # No GUI, use terminal
      "-serial mon:stdio" # Connect serial to stdio
      "-vga none" # No VGA device
    ];
  };

  # A default user able to use sudo
  users.users.guest = {
    isNormalUser = true;
    home = "/home/guest";
    extraGroups = [ "wheel" ];
    initialPassword = "guest";
  };

  security.sudo.wheelNeedsPassword = false;

  services.getty.autologinUser = "guest";

  # services.spice-vdagentd.enable = true;

  services.sshd.enable = true;
  systemd.services.my-go-app = {
    description = "Go App woo";
    wantedBy = [ "multi-user.target" ];
    after = [ "network.target" ]; # if it needs network
    serviceConfig = {
      ExecStart = "${myGoApp}/bin/my-golang-app";
      Restart = "always";
    };
  };
  nixpkgs.config.allowUnfree = true;
  environment.systemPackages = with pkgs; [
    dig
    hey
    neovim
    wget
    #wrk
    myGoApp
  ];

  system.stateVersion = "25.05";
}
