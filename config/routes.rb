Rails.application.routes.draw do
  get "up" => "rails/health#show", as: :rails_health_check

  resource :repository, only: %i[show create destroy]
  resources :reviews, only: %i[new create show] do
    resources :steps, only: :show, controller: "review_steps", param: :step_id
    resources :annotations, only: :create
  end

  root "repositories#show"
end
