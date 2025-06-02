{
  pkgs ? import <nixpkgs> { },
}:

pkgs.buildGoModule {
  # <--- CHANGED!
  pname = "my-go-app"; # The name of your application
  version = "0.1.0"; # Its version
  vendorHash = null;
  src = ./.; # The source directory for your Go program

  # For buildGoPackage, you need to specify the path to the Go package
  # within the 'src' directory. Since main.go is directly in './.',
  # we specify "./."
  goPackagePath = "./."; # <--- ADDED!

  # doNotRecurseIntoSubmodules is not typically used with buildGoPackage
  # It's more relevant for buildGoModule when dealing with vendored dependencies
  # or git submodules.
}
