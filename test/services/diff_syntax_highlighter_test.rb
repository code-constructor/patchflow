require "test_helper"

class DiffSyntaxHighlighterTest < ActiveSupport::TestCase
  test "maps highlighted code to old and new line numbers" do
    diff = <<~DIFF
      diff --git a/example.rb b/example.rb
      --- a/example.rb
      +++ b/example.rb
      @@ -1,2 +1,2 @@
      -class OldName
      +class NewName
       puts "<script>"
    DIFF

    highlights = Patchflow::DiffSyntaxHighlighter.new("example.rb", diff).call

    assert_includes highlights.dig("old", 1), "class"
    assert_includes highlights.dig("new", 1), "NewName"
    assert_includes highlights.dig("new", 2), "&lt;script&gt;"
    assert_not_includes highlights.dig("new", 2), "<script>"
  end
end
