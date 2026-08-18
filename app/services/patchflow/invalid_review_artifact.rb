module Patchflow
  class InvalidReviewArtifact < StandardError
    attr_reader :errors

    def initialize(errors)
      @errors = Array(errors)
      super("Invalid review artifact: #{@errors.join('; ')}")
    end
  end
end
