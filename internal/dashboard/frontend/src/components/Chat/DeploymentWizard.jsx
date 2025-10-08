import React, { useState } from 'react';
import Button from '../shared/Button';
import clsx from 'clsx';
import WorkflowSuggestionCard from './WorkflowSuggestionCard';
import WorkflowDeploymentPanel from './WorkflowDeploymentPanel';

const STEPS = [
  { id: 1, name: 'Describe', title: 'Describe What You Need' },
  { id: 2, name: 'Review', title: 'Review Suggested Workflow' },
  { id: 3, name: 'Configure', title: 'Configure Parameters' },
  { id: 4, name: 'Deploy', title: 'Deploy and Test' }
];

export default function DeploymentWizard({
  initialDescription = '',
  suggestedWorkflows = [],
  onDescriptionSubmit,
  onDeploy,
  onCancel,
  onCustomize
}) {
  const [currentStep, setCurrentStep] = useState(1);
  const [description, setDescription] = useState(initialDescription);
  const [selectedWorkflow, setSelectedWorkflow] = useState(null);
  const [deploymentResult, setDeploymentResult] = useState(null);
  const [loading, setLoading] = useState(false);

  const handleDescriptionSubmit = async (e) => {
    e.preventDefault();

    if (!description.trim()) return;

    setLoading(true);

    try {
      if (onDescriptionSubmit) {
        await onDescriptionSubmit(description);
      }

      setCurrentStep(2);
    } catch (err) {
      console.error('Failed to process description:', err);
    } finally {
      setLoading(false);
    }
  };

  const handleWorkflowSelect = (workflow) => {
    setSelectedWorkflow(workflow);
    setCurrentStep(3);
  };

  const handleDeploy = async (workflowWithParams) => {
    setLoading(true);

    try {
      const result = await onDeploy(workflowWithParams);
      setDeploymentResult(result);
      setCurrentStep(4);
    } catch (err) {
      console.error('Deployment failed:', err);
    } finally {
      setLoading(false);
    }
  };

  const handleBack = () => {
    if (currentStep > 1) {
      setCurrentStep(currentStep - 1);
    }
  };

  const handleRestart = () => {
    setCurrentStep(1);
    setDescription('');
    setSelectedWorkflow(null);
    setDeploymentResult(null);
  };

  return (
    <div className="deployment-wizard bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg shadow-lg p-6 my-4">
      <div className="flex items-center justify-between mb-6">
        <h3 className="text-xl font-bold text-gray-900 dark:text-white">
          Workflow Deployment Wizard
        </h3>
        <button
          onClick={onCancel}
          className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-200 p-1"
          title="Close wizard"
        >
          <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>

      <div className="mb-8">
        <div className="flex items-center justify-between">
          {STEPS.map((step, index) => (
            <React.Fragment key={step.id}>
              <div className="flex flex-col items-center flex-1">
                <div className={clsx(
                  'w-10 h-10 rounded-full flex items-center justify-center font-semibold text-sm transition-colors',
                  currentStep >= step.id
                    ? 'bg-blue-600 text-white'
                    : 'bg-gray-200 dark:bg-gray-700 text-gray-600 dark:text-gray-400'
                )}>
                  {currentStep > step.id ? (
                    <svg className="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                    </svg>
                  ) : (
                    step.id
                  )}
                </div>
                <div className="text-xs font-medium text-gray-600 dark:text-gray-400 mt-2 text-center">
                  {step.name}
                </div>
              </div>
              {index < STEPS.length - 1 && (
                <div className={clsx(
                  'flex-1 h-1 mx-2 rounded transition-colors',
                  currentStep > step.id
                    ? 'bg-blue-600'
                    : 'bg-gray-200 dark:bg-gray-700'
                )} />
              )}
            </React.Fragment>
          ))}
        </div>
      </div>

      <div className="wizard-content min-h-[300px]">
        {currentStep === 1 && (
          <div className="step-content">
            <h4 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">
              {STEPS[0].title}
            </h4>
            <p className="text-sm text-gray-600 dark:text-gray-400 mb-4">
              Describe the workflow or automation you want to create. Be as specific as possible about your requirements.
            </p>
            <form onSubmit={handleDescriptionSubmit}>
              <textarea
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder="Example: I need to monitor my GitHub repository for new pull requests, analyze the code for security issues using Claude, and send a summary to Slack..."
                className="w-full px-4 py-3 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-900 text-gray-900 dark:text-white focus:ring-2 focus:ring-blue-500 focus:border-transparent resize-vertical min-h-[150px]"
                required
              />
              <div className="flex items-center justify-between mt-4">
                <Button
                  type="button"
                  onClick={onCancel}
                  variant="ghost"
                >
                  Cancel
                </Button>
                <Button
                  type="submit"
                  variant="primary"
                  disabled={!description.trim() || loading}
                >
                  {loading ? (
                    <>
                      <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-white mr-2" />
                      Analyzing...
                    </>
                  ) : (
                    <>
                      Next
                      <svg className="w-4 h-4 ml-2" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                      </svg>
                    </>
                  )}
                </Button>
              </div>
            </form>
          </div>
        )}

        {currentStep === 2 && (
          <div className="step-content">
            <h4 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">
              {STEPS[1].title}
            </h4>
            <p className="text-sm text-gray-600 dark:text-gray-400 mb-4">
              Select a workflow template that best matches your requirements, or let us generate a custom workflow.
            </p>

            {suggestedWorkflows.length > 0 ? (
              <div className="space-y-3 max-h-[400px] overflow-y-auto">
                {suggestedWorkflows.map(workflow => (
                  <WorkflowSuggestionCard
                    key={workflow.id}
                    template={workflow}
                    onUse={handleWorkflowSelect}
                  />
                ))}
              </div>
            ) : (
              <div className="text-center py-8">
                <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600 mx-auto mb-4" />
                <p className="text-sm text-gray-600 dark:text-gray-400">
                  Generating workflow suggestions...
                </p>
              </div>
            )}

            <div className="flex items-center justify-between mt-6">
              <Button
                onClick={handleBack}
                variant="ghost"
              >
                <svg className="w-4 h-4 mr-2" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
                </svg>
                Back
              </Button>
            </div>
          </div>
        )}

        {currentStep === 3 && selectedWorkflow && (
          <div className="step-content">
            <h4 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">
              {STEPS[2].title}
            </h4>
            <WorkflowDeploymentPanel
              workflow={selectedWorkflow}
              onDeploy={handleDeploy}
              onCustomize={onCustomize}
              onCancel={handleBack}
            />
          </div>
        )}

        {currentStep === 4 && deploymentResult && (
          <div className="step-content">
            <h4 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">
              {STEPS[3].title}
            </h4>

            <div className="bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded-lg p-6 mb-6">
              <div className="flex items-center gap-3 mb-4">
                <div className="w-12 h-12 bg-green-100 dark:bg-green-900/40 rounded-full flex items-center justify-center">
                  <svg className="w-6 h-6 text-green-600 dark:text-green-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                  </svg>
                </div>
                <div>
                  <h5 className="text-lg font-semibold text-green-900 dark:text-green-100">
                    Workflow Deployed Successfully!
                  </h5>
                  <p className="text-sm text-green-700 dark:text-green-300">
                    Your workflow is now active and ready to use.
                  </p>
                </div>
              </div>

              {deploymentResult.workflow_id && (
                <div className="space-y-2 text-sm">
                  <div className="flex items-center justify-between py-2 border-t border-green-200 dark:border-green-800">
                    <span className="text-green-800 dark:text-green-200 font-medium">Workflow ID:</span>
                    <code className="bg-green-100 dark:bg-green-900/40 px-2 py-1 rounded text-green-900 dark:text-green-100">
                      {deploymentResult.workflow_id}
                    </code>
                  </div>
                  {deploymentResult.endpoint && (
                    <div className="flex items-center justify-between py-2 border-t border-green-200 dark:border-green-800">
                      <span className="text-green-800 dark:text-green-200 font-medium">Endpoint:</span>
                      <code className="bg-green-100 dark:bg-green-900/40 px-2 py-1 rounded text-green-900 dark:text-green-100 text-xs">
                        {deploymentResult.endpoint}
                      </code>
                    </div>
                  )}
                </div>
              )}
            </div>

            <div className="bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg p-4 mb-6">
              <h5 className="text-sm font-semibold text-blue-900 dark:text-blue-100 mb-2">
                Next Steps
              </h5>
              <ul className="text-sm text-blue-800 dark:text-blue-200 space-y-2">
                <li className="flex items-start gap-2">
                  <span className="text-blue-600 dark:text-blue-400 mt-0.5">1.</span>
                  <span>Test your workflow with sample data</span>
                </li>
                <li className="flex items-start gap-2">
                  <span className="text-blue-600 dark:text-blue-400 mt-0.5">2.</span>
                  <span>Monitor workflow executions in the dashboard</span>
                </li>
                <li className="flex items-start gap-2">
                  <span className="text-blue-600 dark:text-blue-400 mt-0.5">3.</span>
                  <span>Adjust parameters as needed for optimal performance</span>
                </li>
              </ul>
            </div>

            <div className="flex items-center gap-3">
              <Button
                onClick={handleRestart}
                variant="outline"
              >
                Deploy Another
              </Button>
              <Button
                onClick={onCancel}
                variant="primary"
              >
                Done
              </Button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
