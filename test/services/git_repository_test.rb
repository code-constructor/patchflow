require "test_helper"

class GitRepositoryTest < ActiveSupport::TestCase
  test "resolves commits and excludes Patchflow artifacts from changed files" do
    with_feature_repository do |directory|
      repository = Patchflow::GitRepository.new(directory)
      target_sha = repository.resolve_commit("HEAD")
      main_sha = repository.resolve_commit("main")
      base_sha = repository.merge_base(main_sha, target_sha)

      assert_equal main_sha, base_sha
      assert_equal %w[app/controllers/sessions_controller.rb app/models/account.rb test/models/account_test.rb], repository.changed_files(base_sha, target_sha).pluck("path").sort
      assert_includes repository.diff_for(base_sha, target_sha, "app/models/account.rb"), "def locked?"
    end
  end

  test "rejects traversal paths before invoking Git" do
    with_feature_repository do |directory|
      repository = Patchflow::GitRepository.new(directory)
      base_sha = repository.resolve_commit("main")
      target_sha = repository.resolve_commit("HEAD")

      assert_raises(Patchflow::InvalidGitPath) do
        repository.diff_for(base_sha, target_sha, "../outside")
      end
    end
  end
end
