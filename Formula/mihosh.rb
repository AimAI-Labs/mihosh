class Mihosh < Formula
  desc "Terminal UI management tool for mihomo"
  homepage "https://github.com/AimAI-Labs/mihosh"
  version "2.0.0-rc.4"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/AimAI-Labs/mihosh/releases/download/v2.0.0-rc.4/mihosh-darwin-arm64.tar.gz"
      sha256 "0f38e4658363ad903cc02635001a3469c5dc21c27f9ff42a69f8637d3510998d"
    else
      url "https://github.com/AimAI-Labs/mihosh/releases/download/v2.0.0-rc.4/mihosh-darwin-amd64.tar.gz"
      sha256 "bcd0bcb8aacfd8cd73e6a9e32aa85e7e7f0cd5328f2969c914172da4197df9b2"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/AimAI-Labs/mihosh/releases/download/v2.0.0-rc.4/mihosh-linux-arm64.tar.gz"
      sha256 "0406641212a9f53724942507e073bf6e4d2719d74c4d8c533eb77479ec899803"
    else
      url "https://github.com/AimAI-Labs/mihosh/releases/download/v2.0.0-rc.4/mihosh-linux-amd64.tar.gz"
      sha256 "7ace71fa0f98c925e7c1d817ab2f488a78ec20b9a59c5064bff99354373f39b0"
    end
  end

  def install
    bin.install "mihosh"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/mihosh version")
  end
end
