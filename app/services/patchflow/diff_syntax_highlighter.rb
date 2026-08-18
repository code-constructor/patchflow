module Patchflow
  class DiffSyntaxHighlighter
    HUNK_HEADER = /\A@@ -(?<old_line>\d+)(?:,\d+)? \+(?<new_line>\d+)(?:,\d+)? @@/

    def initialize(path, diff)
      @diff = diff
      @formatter = Rouge::Formatters::HTMLInline.new("github")
      @lexer = Rouge::Lexer.guess(filename: path)
    rescue Rouge::Guesser::Ambiguous
      @lexer = Rouge::Lexers::PlainText.new
    end

    def call
      old_lines = {}
      new_lines = {}
      old_number = nil
      new_number = nil

      @diff.each_line(chomp: true) do |line|
        if (match = line.match(HUNK_HEADER))
          old_number = match[:old_line].to_i
          new_number = match[:new_line].to_i
          next
        end
        next unless old_number && new_number

        content = line.byteslice(1..).to_s
        case line.first
        when " "
          highlighted = highlight(content)
          old_lines[old_number] = highlighted
          new_lines[new_number] = highlighted
          old_number += 1
          new_number += 1
        when "-"
          old_lines[old_number] = highlight(content)
          old_number += 1
        when "+"
          new_lines[new_number] = highlight(content)
          new_number += 1
        end
      end

      { "old" => old_lines, "new" => new_lines }
    end

    private

    def highlight(source)
      @formatter.format(@lexer.lex(source))
    end
  end
end
