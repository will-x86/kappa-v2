{
  #lib,
  #config,
  pkgs,
  ...
}:
let
  myGoApp = import ./my-go-app { inherit pkgs; };
in
{
  i18n.defaultLocale = "en_US.UTF-8";

  virtualisation.vmVariant = {
    virtualisation.qemu.options = [
      "-nographic"
      "-serial mon:stdio"
      "-vga none"
    ];
  };

  users.users.guest = {
    isNormalUser = true;
    home = "/home/guest";
    extraGroups = [ "wheel" ];
    initialPassword = "guest";
  };

  security.sudo.wheelNeedsPassword = false;

  services.getty.autologinUser = "guest";

  services.sshd.enable = true;
  systemd.services.my-go-app = {
    description = "Go App woo";
    wantedBy = [ "multi-user.target" ];
    after = [ "network.target" ];
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
    myGoApp
  ];

  system.stateVersion = "25.05";
}
