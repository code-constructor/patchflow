class AnnotationsController < ApplicationController
  before_action :require_repository

  def create
    store = Patchflow::ReviewStore.new(current_repository.root)
    artifact = store.find(params[:review_id])
    annotation = build_annotation
    artifact.data.fetch("annotations") << annotation
    artifact.data["updated_at"] = Time.now.utc.iso8601
    store.update(artifact)

    redirect_to annotation_return_path(artifact), notice: "Annotation saved."
  rescue Patchflow::InvalidReviewArtifact => error
    redirect_to review_path(params[:review_id]), alert: error.errors.join(" ")
  rescue Patchflow::ReviewNotFound
    redirect_to root_path, alert: "Review not found."
  end

  private

  def build_annotation
    attributes = params.expect(annotation: %i[scope body file_path side start_line end_line]).to_h
    annotation = {
      "id" => "note-#{Time.now.utc.strftime('%Y%m%d%H%M%S')}-#{SecureRandom.hex(2)}",
      "scope" => attributes.fetch("scope"),
      "body" => attributes.fetch("body"),
      "created_at" => Time.now.utc.iso8601
    }

    annotation["file_path"] = attributes["file_path"] if attributes["file_path"].present?
    annotation["side"] = attributes["side"] if attributes["side"].present?
    annotation["start_line"] = Integer(attributes["start_line"], exception: false) if attributes["start_line"].present?
    annotation["end_line"] = Integer(attributes["end_line"], exception: false) if attributes["end_line"].present?
    annotation
  end

  def annotation_return_path(artifact)
    step_id = params[:step_id]
    return review_step_path(artifact.id, step_id) if artifact.steps.any? { |step| step.fetch("id") == step_id }

    review_path(artifact.id)
  end
end
