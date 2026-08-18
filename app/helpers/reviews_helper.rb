module ReviewsHelper
  def short_sha(sha)
    sha.first(10)
  end

  def annotations_for(artifact, scope:, file_path: nil)
    artifact.annotations.select do |annotation|
      annotation["scope"] == scope && (file_path.nil? || annotation["file_path"] == file_path)
    end
  end
end
