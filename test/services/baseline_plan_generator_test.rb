require "test_helper"

class BaselinePlanGeneratorTest < ActiveSupport::TestCase
  test "places generated and vendored files in a low priority step" do
    files = [
      { "path" => "app/models/account.rb" },
      { "path" => "vendor/javascript/mermaid.standalone.js" },
      { "path" => "package.lock" }
    ]

    steps = Patchflow::BaselinePlanGenerator.new.generate(files)
    generated_step = steps.find { |step| step.fetch("id") == "generated" }

    assert_equal "low", generated_step.fetch("priority")
    assert_equal %w[package.lock vendor/javascript/mermaid.standalone.js], generated_step.fetch("files")
  end
end
