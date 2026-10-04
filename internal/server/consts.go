package server

// inputSchema describes the text and file request, including exclusive question variants and closed defined objects.
const inputSchema = `{
  "type": "object",
  "properties": {
    "objects": {
      "type": "array",
      "minItems": 1,
      "items": {
        "type": "object",
        "properties": {
          "id": {
            "type": "string",
            "minLength": 1,
            "description": "Caller ID, unique within this batch; returned unchanged."
          },
          "source": {
            "oneOf": [
              {
                "type": "object",
                "properties": {
                  "type": {
                    "const": "text"
                  },
                  "text": {
                    "type": "string",
                    "description": "Caller-supplied text, including an empty string."
                  }
                },
                "required": [
                  "type",
                  "text"
                ],
                "additionalProperties": false
              },
              {
                "type": "object",
                "properties": {
                  "type": {
                    "const": "file"
                  },
                  "path": {
                    "type": "string",
                    "description": "Local UTF-8 path, absolute or relative to the server's working directory."
                  },
                  "lines": {
                    "type": "object",
                    "properties": {
                      "start": {
                        "type": "integer",
                        "description": "First included line, starting at 1."
                      },
                      "end": {
                        "type": "integer",
                        "description": "Last included line; start <= end and both lines must exist."
                      }
                    },
                    "required": [
                      "start",
                      "end"
                    ],
                    "additionalProperties": false,
                    "description": "Inclusive 1-based range; omit to read the whole file."
                  }
                },
                "required": [
                  "type",
                  "path"
                ],
                "additionalProperties": false
              }
            ],
            "description": "Inline text or a local UTF-8 file, optionally limited to a line range."
          }
        },
        "required": [
          "id",
          "source"
        ],
        "additionalProperties": false
      },
      "description": "Independent items; outcomes retain input order, even when every item fails."
    },
    "task": {
      "type": "string",
      "minLength": 1,
      "description": "Common task context supplied with each object's content."
    },
    "questions": {
      "type": "object",
      "additionalProperties": {
        "oneOf": [
          {
            "type": "object",
            "properties": {
              "type": {
                "type": "string",
                "enum": [
                  "choice"
                ],
                "description": "Select one category from criteria."
              },
              "instructions": {
                "anyOf": [
                  {
                    "type": "string"
                  },
                  {
                    "type": "object"
                  },
                  {
                    "type": "array"
                  }
                ]
              },
              "criteria": {
                "type": "object",
                "additionalProperties": {
                  "anyOf": [
                    {
                      "type": "string"
                    },
                    {
                      "type": "object"
                    },
                    {
                      "type": "array"
                    },
                    {
                      "type": "null"
                    }
                  ]
                },
                "minProperties": 1,
                "propertyNames": {
                  "type": "string",
                  "minLength": 1
                },
                "description": "Category names mapped to descriptions; null uses the category name alone."
              }
            },
            "required": [
              "type",
              "instructions",
              "criteria"
            ],
            "additionalProperties": false
          },
          {
            "type": "object",
            "properties": {
              "type": {
                "type": "string",
                "enum": [
                  "noul"
                ],
                "description": "Estimate condition truth probability in [0, 1], not a boolean."
              },
              "instructions": {
                "anyOf": [
                  {
                    "type": "string"
                  },
                  {
                    "type": "object"
                  },
                  {
                    "type": "array"
                  }
                ]
              },
              "criteria": {
                "type": "object",
                "properties": {
                  "true": {
                    "anyOf": [
                      {
                        "type": "string"
                      },
                      {
                        "type": "object"
                      },
                      {
                        "type": "array"
                      }
                    ]
                  },
                  "false": {
                    "anyOf": [
                      {
                        "type": "string"
                      },
                      {
                        "type": "object"
                      },
                      {
                        "type": "array"
                      }
                    ]
                  }
                },
                "required": [
                  "true",
                  "false"
                ],
                "additionalProperties": false,
                "description": "Optional descriptions for the condition being true or false."
              }
            },
            "required": [
              "type",
              "instructions"
            ],
            "additionalProperties": false
          },
          {
            "type": "object",
            "properties": {
              "type": {
                "type": "string",
                "enum": [
                  "score"
                ],
                "description": "Numeric score on criteria indices [0, len(criteria)-1]; may be fractional."
              },
              "instructions": {
                "anyOf": [
                  {
                    "type": "string"
                  },
                  {
                    "type": "object"
                  },
                  {
                    "type": "array"
                  }
                ]
              },
              "criteria": {
                "type": "array",
                "items": {
                  "anyOf": [
                    {
                      "type": "string"
                    },
                    {
                      "type": "object"
                    },
                    {
                      "type": "array"
                    }
                  ]
                },
                "minItems": 1,
                "description": "Ordered level descriptions, not score values; zero-based indices define the scale."
              }
            },
            "required": [
              "type",
              "instructions",
              "criteria"
            ],
            "additionalProperties": false
          }
        ]
      },
      "minProperties": 1,
      "propertyNames": {
        "type": "string",
        "minLength": 1
      },
      "description": "Every object receives all questions. Questions are independent and do not use other answers."
    },
    "result_mode": {
      "type": "string",
      "enum": [
        "compact",
        "full"
      ],
      "description": "compact (default): Choice returns choice/probability and optional confidence; ` +
	`Noul returns noul; Score returns score and optional confidence. ` +
	`full adds probabilities to Choice and Score. ` +
	`Confidence reflects provider uncertainty, not probability of correctness."
    }
  },
  "required": [
    "objects",
    "task",
    "questions"
  ],
  "additionalProperties": false
}`

// outputSchema describes success and atomic failure entries, with compact and full assessment alternatives.
const outputSchema = `{
  "type": "object",
  "properties": {
    "results": {
      "type": "array",
      "minItems": 1,
      "items": {
        "oneOf": [
          {
            "type": "object",
            "properties": {
              "id": {
                "type": "string",
                "description": "Input object ID."
              },
              "answers": {
                "type": "object",
                "minProperties": 1,
                "additionalProperties": {
                  "oneOf": [
                    {
                      "type": "object",
                      "properties": {
                        "choice": {
                          "type": "string",
                          "description": "Provider-selected category from the question's criteria."
                        },
                        "probability": {
                          "type": "number",
                          "minimum": 0,
                          "maximum": 1,
                          "description": "Provider probability of the selected category, in [0, 1]."
                        },
                        "confidence": {
                          "$ref": "#/$defs/confidence"
                        }
                      },
                      "required": [
                        "choice",
                        "probability"
                      ],
                      "additionalProperties": false
                    },
                    {
                      "type": "object",
                      "properties": {
                        "choice": {
                          "type": "string",
                          "description": "Provider-selected category from the question's criteria."
                        },
                        "probability": {
                          "type": "number",
                          "minimum": 0,
                          "maximum": 1,
                          "description": "Provider probability of the selected category, in [0, 1]."
                        },
                        "confidence": {
                          "$ref": "#/$defs/confidence"
                        },
                        "probabilities": {
                          "type": "object",
                          "minProperties": 1,
                          "additionalProperties": {
                            "type": "number",
                            "minimum": 0,
                            "maximum": 1
                          },
                          "description": "Category-to-probability map for all criteria categories."
                        }
                      },
                      "required": [
                        "choice",
                        "probability",
                        "probabilities"
                      ],
                      "additionalProperties": false
                    },
                    {
                      "type": "object",
                      "properties": {
                        "noul": {
                          "type": "number",
                          "minimum": 0,
                          "maximum": 1,
                          "description": "Probability that the condition is true, in [0, 1]; not a boolean."
                        }
                      },
                      "required": [
                        "noul"
                      ],
                      "additionalProperties": false
                    },
                    {
                      "type": "object",
                      "properties": {
                        "score": {
                          "$ref": "#/$defs/score"
                        },
                        "confidence": {
                          "$ref": "#/$defs/confidence"
                        }
                      },
                      "required": [
                        "score"
                      ],
                      "additionalProperties": false
                    },
                    {
                      "type": "object",
                      "properties": {
                        "score": {
                          "$ref": "#/$defs/score"
                        },
                        "confidence": {
                          "$ref": "#/$defs/confidence"
                        },
                        "probabilities": {
                          "type": "object",
                          "minProperties": 1,
                          "additionalProperties": {
                            "type": "number",
                            "minimum": 0,
                            "maximum": 1
                          },
                          "description": "Probabilities for every zero-based level index, using decimal-string keys."
                        }
                      },
                      "required": [
                        "score",
                        "probabilities"
                      ],
                      "additionalProperties": false
                    }
                  ]
                },
                "description": "One assessment per requested question, keyed by question ID."
              }
            },
            "required": [
              "id",
              "answers"
            ],
            "additionalProperties": false
          },
          {
            "type": "object",
            "properties": {
              "id": {
                "type": "string",
                "description": "Input object ID."
              },
              "error": {
                "type": "string",
                "minLength": 1,
                "description": "Concrete cause for this object; other outcomes stay independent."
              }
            },
            "required": [
              "id",
              "error"
            ],
            "additionalProperties": false
          }
        ]
      },
      "description": "Input-ordered outcomes; individual failures never fail the batch."
    }
  },
  "required": [
    "results"
  ],
  "additionalProperties": false,
  "$defs": {
    "confidence": {
      "type": "number",
      "minimum": 0,
      "maximum": 1,
      "description": "Provider confidence based on uncertainty, not the probability of a correct answer."
    },
    "score": {
      "type": "number",
      "minimum": 0,
      "description": "Provider score on criteria indices [0, len(criteria)-1]; may be fractional."
    }
  }
}`

// Optional assessment field names are shared by the projection serializers.
const (
	fieldConfidence    = "confidence"
	fieldProbabilities = "probabilities"
)
