{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-parts.url = "github:hercules-ci/flake-parts";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = inputs @ {
    nixpkgs,
    flake-parts,
    flake-utils,
    ...
  }:
    flake-parts.lib.mkFlake {inherit inputs;} {
      systems = flake-utils.lib.defaultSystems;

      perSystem = {pkgs, ...}: let
        runtimeDeps = [pkgs.mpv];
        nihongoPkg = pkgs.buildGoModule {
          pname = "nihongo";
          version = "1.0";
          src = pkgs.lib.cleanSource ./.;
          vendorHash = "sha256-j6zMVJkzNK+s07SowF3FVt5DwgnQPWctZtpljRBGi50=";

          buildInputs = runtimeDeps;
          nativeBuildInputs = [pkgs.go pkgs.makeWrapper] ++ runtimeDeps;
          nativeCheckInputs = [pkgs.go pkgs.makeWrapper] ++ runtimeDeps;

          buildPhase = ''
            go build -o nihongo .
          '';

          installPhase = ''
            install -Dm755 nihongo $out/bin/nihongo
            wrapProgram $out/bin/nihongo --prefix PATH : ${pkgs.lib.makeBinPath runtimeDeps}
          '';
        };
      in {
        packages.default = nihongoPkg;

        devShells.default = pkgs.mkShell {
          buildInputs = [pkgs.go] ++ runtimeDeps;
        };

        apps.default = flake-utils.lib.mkApp {
          drv = nihongoPkg;
        };
      };
    };
}
