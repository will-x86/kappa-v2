{
  pkgs ? import <nixpkgs> { },
}:

pkgs.buildGoModule {
  pname = "my-go-app";
  version = "0.1.0";
  vendorHash = null;
  src = ./.;

  goPackagePath = "./.";
}
