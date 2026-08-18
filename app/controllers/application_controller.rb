class ApplicationController < ActionController::Base
  # Only allow modern browsers supporting webp images, web push, badges, import maps, CSS nesting, and CSS :has.
  allow_browser versions: :modern

  # Changes to the importmap will invalidate the etag for HTML responses
  stale_when_importmap_changes

  helper_method :current_repository

  private

  def current_repository
    return unless session[:repository_path]

    @current_repository ||= Patchflow::GitRepository.new(session[:repository_path])
  end

  def require_repository
    return if current_repository

    redirect_to root_path, alert: "Choose a repository first."
  rescue Patchflow::InvalidRepository => error
    session.delete(:repository_path)
    redirect_to root_path, alert: error.message
  end
end
