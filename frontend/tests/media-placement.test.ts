import { test } from 'node:test'
import assert from 'node:assert/strict'

import { placeMedia } from '../src/lib/media-placement'
import type { MediaSegment } from '../src/lib/media-placement'

function text(value: string): MediaSegment {
  return { type: 'text', text: value }
}

function video(): MediaSegment {
  return { type: 'video' }
}

function image(index: number): MediaSegment {
  return { type: 'image', index }
}

test('placeMedia returns a single trimmed text segment when there is no media', () => {
  assert.deepEqual(placeMedia('P1\nP2', { hasVideo: false, imageCount: 0 }), [text('P1\nP2')])
})

test('placeMedia returns an empty list for empty text without media', () => {
  assert.deepEqual(placeMedia('', { hasVideo: false, imageCount: 0 }), [])
  assert.deepEqual(placeMedia('   \n  ', { hasVideo: false, imageCount: 0 }), [])
})

test('placeMedia trims leading and trailing newlines without media', () => {
  assert.deepEqual(placeMedia('\nP1\nP2\n', { hasVideo: false, imageCount: 0 }), [
    text('P1\nP2'),
  ])
})

test('placeMedia keeps blank lines inside the text segment without media', () => {
  assert.deepEqual(placeMedia('P1\n\nP2', { hasVideo: false, imageCount: 0 }), [
    text('P1\n\nP2'),
  ])
  assert.deepEqual(placeMedia('\n\nP1\n\nP2\n\n', { hasVideo: false, imageCount: 0 }), [
    text('P1\n\nP2'),
  ])
})

const videoOnlyCases: Array<{ name: string; text: string; expected: MediaSegment[] }> = [
  {
    name: 'one paragraph keeps the text before the video',
    text: 'A',
    expected: [text('A'), video()],
  },
  {
    name: 'two paragraphs split one and one',
    text: 'A\nB',
    expected: [text('A'), video(), text('B')],
  },
  {
    name: 'three paragraphs split two and one',
    text: 'A\nB\nC',
    expected: [text('A\nB'), video(), text('C')],
  },
  {
    name: 'six paragraphs split three and three',
    text: 'P1\nP2\nP3\nP4\nP5\nP6',
    expected: [text('P1\nP2\nP3'), video(), text('P4\nP5\nP6')],
  },
]

for (const { name, text: input, expected } of videoOnlyCases) {
  test(`placeMedia video only: ${name}`, () => {
    assert.deepEqual(placeMedia(input, { hasVideo: true, imageCount: 0 }), expected)
  })
}

test('placeMedia spreads two images through four paragraphs', () => {
  assert.deepEqual(placeMedia('P1\nP2\nP3\nP4', { hasVideo: false, imageCount: 2 }), [
    text('P1'),
    image(0),
    text('P2\nP3'),
    image(1),
    text('P4'),
  ])
})

test('placeMedia interleaves the video between two images in four paragraphs', () => {
  assert.deepEqual(placeMedia('P1\nP2\nP3\nP4', { hasVideo: true, imageCount: 2 }), [
    text('P1'),
    image(0),
    text('P2'),
    video(),
    text('P3'),
    image(1),
    text('P4'),
  ])
})

test('placeMedia keeps images sharing one gap together in index order', () => {
  assert.deepEqual(placeMedia('P1', { hasVideo: false, imageCount: 3 }), [
    text('P1'),
    image(0),
    image(1),
    image(2),
  ])
})

test('placeMedia puts the video before the image on a shared gap', () => {
  assert.deepEqual(placeMedia('P1', { hasVideo: true, imageCount: 1 }), [
    text('P1'),
    video(),
    image(0),
  ])
})

test('placeMedia puts the video before the image on a shared gap with two paragraphs', () => {
  assert.deepEqual(placeMedia('P1\nP2', { hasVideo: true, imageCount: 1 }), [
    text('P1'),
    video(),
    image(0),
    text('P2'),
  ])
})

test('placeMedia spreads more images than paragraphs', () => {
  assert.deepEqual(placeMedia('P1\nP2', { hasVideo: false, imageCount: 5 }), [
    text('P1'),
    image(0),
    image(1),
    image(2),
    image(3),
    text('P2'),
    image(4),
  ])
})

test('placeMedia shows the video then the images for empty text', () => {
  assert.deepEqual(placeMedia('', { hasVideo: true, imageCount: 2 }), [
    video(),
    image(0),
    image(1),
  ])
})

test('placeMedia shows only images for empty text without a video', () => {
  assert.deepEqual(placeMedia('', { hasVideo: false, imageCount: 2 }), [image(0), image(1)])
})

test('placeMedia shows the video alone for empty text without images', () => {
  assert.deepEqual(placeMedia('', { hasVideo: true, imageCount: 0 }), [video()])
})

test('placeMedia keeps blank lines inside text segments but not at their edges', () => {
  assert.deepEqual(placeMedia('P1\n\nP2\n\nP3', { hasVideo: true, imageCount: 0 }), [
    text('P1\n\nP2'),
    video(),
    text('P3'),
  ])
})

test('placeMedia keeps blank lines inside the middle segment with images', () => {
  assert.deepEqual(
    placeMedia('P1\n\nP2\n\nP3\n\nP4', { hasVideo: false, imageCount: 2 }),
    [text('P1'), image(0), text('P2\n\nP3'), image(1), text('P4')],
  )
})

test('placeMedia ignores whitespace-only lines when counting paragraphs', () => {
  const segments = placeMedia('A\n   \nB\n\t\nC', { hasVideo: true, imageCount: 0 })

  assert.equal(segments.length, 3)
  assert.deepEqual(segments[1], video())
  const before = segments[0]
  const after = segments[2]
  assert.equal(before.type, 'text')
  assert.equal(after.type, 'text')
  if (before.type === 'text' && after.type === 'text') {
    assert.ok(before.text.includes('A'), `expected A before the video in: ${JSON.stringify(before.text)}`)
    assert.ok(before.text.includes('B'), `expected B before the video in: ${JSON.stringify(before.text)}`)
    assert.ok(!before.text.includes('C'), `expected C after the video, got: ${JSON.stringify(before.text)}`)
    assert.ok(after.text.includes('C'), `expected C after the video in: ${JSON.stringify(after.text)}`)
    assert.ok(!after.text.includes('A'), `expected A before the video, got: ${JSON.stringify(after.text)}`)
  }
})

test('placeMedia image segments use zero-based indexes in order', () => {
  const segments = placeMedia('P1\nP2\nP3\nP4', { hasVideo: false, imageCount: 2 })
  const images = segments.filter((segment: MediaSegment) => segment.type === 'image')

  assert.deepEqual(images, [image(0), image(1)])
})
