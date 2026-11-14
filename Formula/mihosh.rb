class Mihosh < Formula
  desc "Terminal UI management tool for mihomo"
  homepage "https://github.com/AimAI-Labs/mihosh"
  version "2.0.0-rc.2"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/AimAI-Labs/mihosh/releases/download/v2.0.0-rc.2/mihosh-darwin-arm64.tar.gz"
      sha256 "96327ebb55ae549641ae95d89d88e1f7f3cbaa5af4535d95574cdeb6af951b97"
    else
      url "https://github.com/AimAI-Labs/mihosh/releases/download/v2.0.0-rc.2/mihosh-darwin-amd64.tar.gz"
      sha256 "a7d1a126ff4f761d8b09a34179afd48c71ba3d7bbf43f0ff7d36a47c73eecd10"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/AimAI-Labs/mihosh/releases/download/v2.0.0-rc.2/mihosh-linux-arm64.tar.gz"
      sha256 "a824e99e9ec9b394e90c1f210607aa6062fe9bef736873c773b275455db22351"
    else
      url "https://github.com/AimAI-Labs/mihosh/releases/download/v2.0.0-rc.2/mihosh-linux-amd64.tar.gz"
      sha256 "f2473965bbf0bf3d1eeaa5983a113a66a144d9b24215ee6501e1cbf1eb05791e"
    end
  end

  def install
    bin.install "mihosh"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/mihosh version")
  end
end
