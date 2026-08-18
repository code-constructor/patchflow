module Patchflow
  class BaselinePlanGenerator
    CATEGORY_RULES = [
      {
        id: "security",
        title: "Inspect security-sensitive behavior",
        priority: "critical",
        rationale: "Authentication, authorization, session, permission, or credential paths can change the trust boundary.",
        patterns: %r{(?:auth|session|permission|policy|policies|security|credential|password|token)}i
      },
      {
        id: "domain",
        title: "Understand domain behavior",
        priority: "high",
        rationale: "Domain models and services define the behavior that supporting layers depend on.",
        patterns: %r{\A(?:app/(?:models|services)|lib|domain)/}
      },
      {
        id: "data",
        title: "Follow data and configuration changes",
        priority: "high",
        rationale: "Persistence and configuration changes shape the state available to the application.",
        patterns: %r{\A(?:db/|config/)|(?:schema|migration)}i
      },
      {
        id: "interfaces",
        title: "Trace entry points and interfaces",
        priority: "high",
        rationale: "Routes, controllers, and public interfaces show how the change enters and leaves the system.",
        patterns: %r{\A(?:app/controllers|config/routes|app/(?:graphql|channels)|api)/}
      },
      {
        id: "execution",
        title: "Trace background and command execution",
        priority: "medium",
        rationale: "Jobs, commands, and executables reveal asynchronous or operational effects.",
        patterns: %r{\A(?:app/jobs|bin|script|lib/tasks)/}
      },
      {
        id: "presentation",
        title: "Review presentation changes",
        priority: "low",
        rationale: "Views, styles, and browser behavior are easiest to assess after their supporting behavior is understood.",
        patterns: %r{\A(?:app/views|app/helpers|app/assets|app/javascript|public)/}
      },
      {
        id: "tests",
        title: "Verify the intended behavior",
        priority: "medium",
        rationale: "Tests document expected behavior and expose missing cases after the implementation path is understood.",
        patterns: %r{\A(?:test|spec)/}
      },
      {
        id: "generated",
        title: "Scan generated and vendored files",
        priority: "low",
        rationale: "Generated, minified, and vendored files are usually better verified through their source or generation process than line by line.",
        patterns: %r{(?:\A(?:vendor|node_modules)/|(?:\.min\.(?:css|js)|\.lock)\z)}i
      },
      {
        id: "supporting",
        title: "Review supporting changes",
        priority: "low",
        rationale: "These files support the change but do not match a more important execution or domain path.",
        patterns: %r{.*}
      }
    ].freeze

    def generate(changed_files)
      remaining = changed_files.map { |file| file.fetch("path") }

      CATEGORY_RULES.filter_map do |category|
        matches, remaining = remaining.partition { |path| path.match?(category.fetch(:patterns)) }
        next if matches.empty?

        {
          "id" => category.fetch(:id),
          "title" => category.fetch(:title),
          "priority" => category.fetch(:priority),
          "rationale" => category.fetch(:rationale),
          "files" => matches.sort
        }
      end
    end
  end
end
