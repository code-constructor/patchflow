class RepositoriesController < ApplicationController
  def show
    @repository_path = current_repository&.root || Rails.root
    @reviews = current_repository ? Patchflow::ReviewStore.new(current_repository.root).all : []
  rescue Patchflow::InvalidRepository, Patchflow::UnsafeReviewPath => error
    session.delete(:repository_path)
    @repository_path = Rails.root
    @reviews = []
    flash.now[:alert] = error.message
  end

  def create
    repository = Patchflow::GitRepository.new(params[:repository_path])
    session[:repository_path] = repository.root.to_s
    redirect_to new_review_path, notice: "Opened #{repository.name}."
  rescue Patchflow::InvalidRepository => error
    @repository_path = params[:repository_path]
    @reviews = []
    flash.now[:alert] = error.message
    render :show, status: :unprocessable_entity
  end

  def destroy
    session.delete(:repository_path)
    redirect_to root_path, notice: "Repository closed."
  end
end
