import { z } from 'zod';

export const DECISION_MAX_QUESTIONS = 16;
export const DECISION_MAX_OPTIONS = 32;
export const DECISION_MAX_SCORE_LEVELS = 10;
export const DECISION_MAX_INPUT_CHARACTERS = 80_000;
export const DECISION_MAX_QUESTION_CHARACTERS = 2_000;
export const DECISION_MAX_LABEL_CHARACTERS = 200;
export const DECISION_IMAGE_MAX_COUNT = 4;
export const DECISION_IMAGE_MAX_DECODED_BYTES = 2 * 1024 * 1024;
export const DECISION_IMAGE_MEDIA_TYPES = [
  'image/png',
  'image/jpeg',
  'image/webp',
] as const;

const BASE64_PATTERN = /^[A-Za-z0-9+/]+={0,2}$/;
const QUESTION_ID_PATTERN = /^[A-Za-z0-9_-]{1,64}$/;

export function decodedBase64ByteLength(data: string): number {
  const padding = data.endsWith('==') ? 2 : data.endsWith('=') ? 1 : 0;
  return Math.floor((data.length * 3) / 4) - padding;
}

function hasDuplicates(values: readonly string[]): boolean {
  return new Set(values).size !== values.length;
}

const questionIdSchema = z.string().regex(QUESTION_ID_PATTERN);
const questionTextSchema = z
  .string()
  .min(1)
  .max(DECISION_MAX_QUESTION_CHARACTERS);
const labelSchema = z.string().min(1).max(DECISION_MAX_LABEL_CHARACTERS);

const predicateQuestionSchema = z
  .object({
    id: questionIdSchema,
    type: z.literal('predicate'),
    question: questionTextSchema,
  })
  .strict();

const choiceQuestionSchema = z
  .object({
    id: questionIdSchema,
    type: z.literal('choice'),
    question: questionTextSchema,
    options: z.array(labelSchema).min(2).max(DECISION_MAX_OPTIONS),
  })
  .strict()
  .refine((question) => !hasDuplicates(question.options), {
    message: 'Choice options must be unique',
    path: ['options'],
  });

const scoreQuestionSchema = z
  .object({
    id: questionIdSchema,
    type: z.literal('score'),
    question: questionTextSchema,
    min: z.number().int(),
    max: z.number().int(),
    labels: z.record(z.string(), labelSchema).optional(),
  })
  .strict()
  .refine(
    (question) =>
      question.max > question.min &&
      question.max - question.min + 1 <= DECISION_MAX_SCORE_LEVELS,
    {
      message: `Score questions need between 2 and ${DECISION_MAX_SCORE_LEVELS} integer levels`,
      path: ['max'],
    },
  )
  .refine(
    (question) =>
      Object.keys(question.labels ?? {}).every((level) => {
        const value = Number(level);
        return (
          Number.isInteger(value) &&
          value >= question.min &&
          value <= question.max
        );
      }),
    {
      message: 'Score labels must name levels inside min..max',
      path: ['labels'],
    },
  );

export const decisionQuestionSchema = z.union([
  predicateQuestionSchema,
  choiceQuestionSchema,
  scoreQuestionSchema,
]);

export const decisionImageSchema = z
  .object({
    mediaType: z.enum(DECISION_IMAGE_MEDIA_TYPES),
    data: z.string().min(1).regex(BASE64_PATTERN),
  })
  .strict()
  .refine(
    (image) =>
      decodedBase64ByteLength(image.data) <= DECISION_IMAGE_MAX_DECODED_BYTES,
    { message: 'Decision images must be at most 2 MiB each', path: ['data'] },
  );

export const decisionRequestSchema = z
  .object({
    input: z.string().max(DECISION_MAX_INPUT_CHARACTERS),
    images: z
      .array(decisionImageSchema)
      .max(DECISION_IMAGE_MAX_COUNT)
      .default([]),
    questions: z
      .array(decisionQuestionSchema)
      .min(1)
      .max(DECISION_MAX_QUESTIONS),
  })
  .strict()
  .refine(
    (request) =>
      !hasDuplicates(request.questions.map((question) => question.id)),
    { message: 'Question ids must be unique', path: ['questions'] },
  );

export type DecisionImage = z.infer<typeof decisionImageSchema>;
export type DecisionQuestion = z.infer<typeof decisionQuestionSchema>;
export type DecisionRequest = z.infer<typeof decisionRequestSchema>;

export type DecisionRequestParse =
  | { ok: true; request: DecisionRequest }
  | { ok: false; message: string };

export function parseDecisionRequest(value: unknown): DecisionRequestParse {
  const parsed = decisionRequestSchema.safeParse(value);
  if (parsed.success) return { ok: true, request: parsed.data };
  const issue = parsed.error.issues[0];
  const where = issue?.path.length ? issue.path.join('.') : 'request';
  return { ok: false, message: `${where}: ${issue?.message ?? 'invalid'}` };
}
