ENV["RAILS_ENV"] ||= "test"
require_relative "../config/environment"
require "rails/test_help"
require "fileutils"
require "open3"
require "tmpdir"

module GitRepositoryTestHelper
  def with_feature_repository
    directory = Dir.mktmpdir("patchflow-test-repository")
    git!(directory, "init", "-b", "main")
    git!(directory, "config", "user.email", "patchflow@example.test")
    git!(directory, "config", "user.name", "Patchflow Test")

    write_repository_file(directory, "app/models/account.rb", "class Account\nend\n")
    git!(directory, "add", ".")
    git!(directory, "commit", "-m", "Initial application")

    git!(directory, "checkout", "-b", "feature/account-locking")
    write_repository_file(directory, "app/models/account.rb", "class Account\n  def locked? = true\nend\n")
    write_repository_file(directory, "app/controllers/sessions_controller.rb", "class SessionsController\nend\n")
    write_repository_file(directory, "test/models/account_test.rb", "# account locking behavior\n")
    write_repository_file(directory, ".patchflow/generated.txt", "must not review me\n")
    git!(directory, "add", ".")
    git!(directory, "commit", "-m", "Add account locking")

    yield directory
  ensure
    FileUtils.remove_entry(directory) if directory && File.exist?(directory)
  end

  def git!(directory, *arguments)
    stdout, stderr, status = Open3.capture3("git", "-C", directory, *arguments)
    raise "git #{arguments.join(' ')} failed: #{stderr}" unless status.success?

    stdout.strip
  end

  def write_repository_file(directory, relative_path, content)
    path = File.join(directory, relative_path)
    FileUtils.mkdir_p(File.dirname(path))
    File.write(path, content)
  end
end

module ActiveSupport
  class TestCase
    # Run tests in parallel with specified workers
    parallelize(workers: :number_of_processors)

    # Setup all fixtures in test/fixtures/*.yml for all tests in alphabetical order.
    fixtures :all
    include GitRepositoryTestHelper
  end
end

class ActionDispatch::IntegrationTest
  include GitRepositoryTestHelper
end
