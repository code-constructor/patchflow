module Patchflow
  class CodeSyntaxHighlighter
    def initialize(path, source, start_line:)
      @source = source
      @start_line = start_line
      @formatter = Rouge::Formatters::HTMLInline.new("github")
      @lexer = Rouge::Lexer.guess(filename: path)
    rescue Rouge::Guesser::Ambiguous
      @lexer = Rouge::Lexers::PlainText.new
    end

    def call
      @source.each_line(chomp: true).each_with_index.map do |line, index|
        {
          "number" => @start_line + index,
          "html" => @formatter.format(@lexer.lex(line))
        }
      end
    end
  end
end
