class ReviewsController < ApplicationController
  before_action :require_repository

  def new
    @base_ref = params[:base_ref].presence || "main"
    @target_ref = params[:target_ref].presence || "HEAD"
  end

  def create
    @base_ref = params[:base_ref].presence || "main"
    @target_ref = params[:target_ref].presence || "HEAD"
    @artifact = Patchflow::ReviewCreator.new(
      repository: current_repository,
      base_ref: @base_ref,
      target_ref: @target_ref
    ).create
    redirect_to review_path(@artifact.id), notice: "Review artifact created."
  rescue Patchflow::GitError, Patchflow::EmptyGitDiff, Patchflow::InvalidReviewArtifact, Patchflow::UnsafeReviewPath => error
    flash.now[:alert] = error.message
    render :new, status: :unprocessable_entity
  end

  def show
    @store = Patchflow::ReviewStore.new(current_repository.root)
    @artifact = @store.find(params[:id])
    @overview = @store.read_overview(@artifact)
    @stale = current_repository.target_changed?(@artifact.source.fetch("target_ref"), @artifact.source.fetch("target_sha"))
  rescue Patchflow::InvalidGitReference
    @stale = false
  rescue Patchflow::ReviewNotFound
    redirect_to root_path, alert: "Review not found."
  end
end
