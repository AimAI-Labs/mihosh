class Mihosh < Formula
  desc "Terminal UI management tool for mihomo"
  homepage "https://github.com/AimAI-Labs/mihosh"
  version "2.0.0-rc.1"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/AimAI-Labs/mihosh/releases/download/v2.0.0-rc.1/mihosh-darwin-arm64.tar.gz"
      sha256 "840a39fdd52a84ba190d024208b80615f3ed025f060d0e4f5fd8ac27577b352e"
    else
      url "https://github.com/AimAI-Labs/mihosh/releases/download/v2.0.0-rc.1/mihosh-darwin-amd64.tar.gz"
      sha256 "657d9178abaf214f23c8289b8662c2875ae80b3bd892e61b982cce7c504ea4b3"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/AimAI-Labs/mihosh/releases/download/v2.0.0-rc.1/mihosh-linux-arm64.tar.gz"
      sha256 "de4411cf3102003a1b8505a64c53985dbbc433d4972bd1d70d8680e16d33844a"
    else
      url "https://github.com/AimAI-Labs/mihosh/releases/download/v2.0.0-rc.1/mihosh-linux-amd64.tar.gz"
      sha256 "baa13f06224507a3d10eea414113bcf05687900c4a2c3f120f2ec3b6a0e7c2ec"
    end
  end

  def install
    bin.install "mihosh"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/mihosh version")
  end
end
