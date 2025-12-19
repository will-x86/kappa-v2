{
  description = "Basic go flake";
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    playwright.url = "github:pietdevries94/playwright-web-flake/1.51.0";
    rust-overlay.url = "github:oxalica/rust-overlay";
    flake-utils.url = "github:numtide/flake-utils";
  };
  outputs =
    {
      self,
      nixpkgs,
      flake-utils,
      rust-overlay,
      playwright,
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        overlays = [
          (import rust-overlay)
          (final: prev: {
            inherit (playwright.packages.${system}) playwright-test playwright-driver;
          })
        ];
        pkgs = import nixpkgs {
          inherit system;
          inherit overlays;
        };
      in
      with pkgs;
      {
        devShell = pkgs.mkShell {
          LD_LIBRARY_PATH = lib.makeLibraryPath [ openssl ];
          buildInputs = with pkgs; [
            go # go
            gopls # Formatter
            gotools
            go-outline
            gopkgs
            godef
            golint
            nodejs
            python3
            python3Packages.pip
            runc
            python3Packages.setuptools
            python312Packages.gyp
            pkg-config # Rust dep
            python312Packages.distutils
            playwright-test # Nextjs tests
            #openssl # Rust dep
            eza
            fd
            rust-bin.stable.latest.default # love stable
            rust-analyzer
            sqlite # DB for rust
            zsh
          ];
          shellHook = ''
            alias ls=eza
            alias find=fd
            export GOPATH=$HOME/go
            export PATH=$GOPATH/bin:$PATH
            export PATH=$PATH:$PWD/node_modules/.bin
            export PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD=1
            export PATH=$PATH:${pkgs.rust-analyzer}/bin
            export PLAYWRIGHT_BROWSERS_PATH="${pkgs.playwright-driver.browsers}"
          '';
        };
      }
    );
}
