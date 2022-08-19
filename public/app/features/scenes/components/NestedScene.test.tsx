import { screen, render } from '@testing-library/react';
import React from 'react';

import { NestedScene } from './NestedScene';
import { Scene } from './Scene';
import { SceneCanvasText } from './SceneCanvasText';
import { SceneFlexLayout } from './SceneFlexLayout';

function setup() {
  const scene = new Scene({
    title: 'Hello',
    children: [
      new SceneFlexLayout({
        children: [
          new NestedScene({
            title: 'Nested title',
            canRemove: true,
            canCollapse: true,
            children: [
              new SceneFlexLayout({
                children: [new SceneCanvasText({ text: 'SceneCanvasText' })],
              }),
            ],
          }),
        ],
      }),
    ],
  });

  render(<scene.Component model={scene} />);
}

describe('NestedScene', () => {
  it('Renders heading and layout', () => {
    setup();
    expect(screen.getByText('Nested title')).toBeInTheDocument();
    expect(screen.getByText('SceneCanvasText')).toBeInTheDocument();
  });

  it('Can remove', async () => {
    setup();
    screen.getByRole('button', { name: 'Remove scene' }).click();
    expect(screen.queryByText('Nested title')).not.toBeInTheDocument();
  });

  it('Can collapse and expand', async () => {
    setup();

    screen.getByRole('button', { name: 'Collapse scene' }).click();
    expect(screen.queryByText('SceneCanvasText')).not.toBeInTheDocument();

    screen.getByRole('button', { name: 'Expand scene' }).click();
    expect(screen.getByText('SceneCanvasText')).toBeInTheDocument();
  });
});
